package invoiceqr

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func TestSuggestPaymentDetailsFromTextFindsClearFields(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	assertSuggestedPaymentDetails(t, suggestion, PaymentDetails{
		Payee:     "ACME BV",
		IBAN:      "BE68 5390 0754 7034",
		Amount:    "42.50",
		Reference: "INV-2026-001",
	})
}

func TestSuggestPaymentDetailsFromTextUsesExplicitOverrides(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{
		Payee:     "Override BV",
		Amount:    "10",
		Reference: "MANUAL-REF",
	})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	assertSuggestedPaymentDetails(t, suggestion, PaymentDetails{
		Payee:     "Override BV",
		IBAN:      "BE68 5390 0754 7034",
		Amount:    "10",
		Reference: "MANUAL-REF",
	})
}

func TestSuggestPaymentDetailsFromTextParsesThousandsSeparatedAmounts(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "belgian", text: "Amount: EUR 1.234,56"},
		{name: "english", text: "Amount: EUR 1,234.56"},
		{name: "integer", text: "Amount: EUR 1.234"},
		{name: "space", text: "Amount: EUR 1 234,56"},
		{name: "non-breaking space", text: "Amount: EUR 1\u00a0234,56"},
		{name: "narrow non-breaking space", text: "Amount: EUR 1\u202f234,56"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
`+tt.text+`
Reference: INV-2026-001
`, PaymentDetails{})

			if err != nil {
				t.Fatalf("expected suggestion, got %v", err)
			}
			if suggestion.Amount != "1234.56" && tt.name != "integer" {
				t.Fatalf("amount = %q, want 1234.56", suggestion.Amount)
			}
			if suggestion.Amount != "1234.00" && tt.name == "integer" {
				t.Fatalf("amount = %q, want 1234.00", suggestion.Amount)
			}
		})
	}
}

func TestSuggestPaymentDetailsFromTextUsesCurrencyAmountAfterDate(t *testing.T) {
	tests := []string{
		"Total due by 2026-06-30: EUR 42.50",
		"Total due by 2026-06-30 EUR 42.50",
		"Total: 42.50 EUR",
	}

	for _, line := range tests {
		t.Run(line, func(t *testing.T) {
			suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
`+line+`
Reference: INV-2026-001
`, PaymentDetails{})

			if err != nil {
				t.Fatalf("expected suggestion, got %v", err)
			}
			if suggestion.Amount != "42.50" {
				t.Fatalf("amount = %q, want 42.50", suggestion.Amount)
			}
		})
	}
}

func TestSuggestPaymentDetailsFromTextFindsPaymentInstructionAmount(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		amount string
	}{
		{name: "dutch", line: "Gelieve € 86,36 te betalen"},
		{name: "english", line: "Please pay EUR 86.36"},
		{name: "english euro after", line: "Please pay 86,36 €"},
		{name: "english before date", line: "Please pay EUR 86.36 2026-06-30"},
		{name: "english before reference", line: "Please pay EUR 86.36 123"},
		{name: "integer before date", line: "Please pay EUR 1000 2026-06-30", amount: "1000.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
`+tt.line+`
Reference: INV-2026-001
`, PaymentDetails{})

			if err != nil {
				t.Fatalf("expected suggestion, got %v", err)
			}
			wantAmount := tt.amount
			if wantAmount == "" {
				wantAmount = "86.36"
			}
			if suggestion.Amount != wantAmount {
				t.Fatalf("amount = %q, want %s", suggestion.Amount, wantAmount)
			}
		})
	}
}

func TestSuggestPaymentDetailsFromTextPrefersPaymentInstructionAmount(t *testing.T) {
	report, err := SuggestPaymentDetailsReportFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Gelieve € 86,36 te betalen
Total amount to pay: EUR 42.50
Total: EUR 12.00
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion report, got %v", err)
	}
	if report.Amount.Value != "86.36" || report.Amount.Evidence != "Gelieve € 86,36 te betalen" {
		t.Fatalf("unexpected selected amount: %+v", report.Amount)
	}
	if len(report.AgentContext.ReviewCandidates.Amount) != 1 {
		t.Fatalf("expected conflicting totals as review candidates, got %+v", report.AgentContext.ReviewCandidates.Amount)
	}
	for _, candidate := range report.AgentContext.ReviewCandidates.Amount {
		if candidate.Reason != "conflicting_generic_total" {
			t.Fatalf("expected conflicting_generic_total reason, got %+v", candidate)
		}
	}
}

func TestSuggestPaymentDetailsFromTextDoesNotSelectPaymentFeeAsInstructionAmount(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{name: "payment fee", line: "Payment fee: EUR 2.50"},
		{name: "late fee after pay wording", line: "Please pay before the due date. Late fee: EUR 2.50"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := SuggestPaymentDetailsReportFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
`+tt.line+`
Total amount to pay: EUR 121.00
Reference: INV-2026-001
`, PaymentDetails{})

			if err != nil {
				t.Fatalf("expected suggestion report, got %v", err)
			}
			if report.Amount.Value != "121.00" || report.Amount.Evidence != "Total amount to pay: EUR 121.00" {
				t.Fatalf("unexpected selected amount: %+v", report.Amount)
			}
			if len(report.AgentContext.Candidates.Amount) != 1 || report.AgentContext.Candidates.Amount[0].Kind != amountCandidateKindPayableTotal {
				t.Fatalf("expected payable-total amount candidate, got %+v", report.AgentContext.Candidates.Amount)
			}
		})
	}
}

func TestSuggestPaymentDetailsFromTextDoesNotTruncateMalformedPaymentInstructionAmount(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{name: "short malformed group", line: "Please pay EUR 12 34"},
		{name: "mixed malformed group", line: "Please pay EUR 1.234 567"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := SuggestPaymentDetailsReportFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
`+tt.line+`
Total amount to pay: EUR 121.00
Reference: INV-2026-001
`, PaymentDetails{})

			if err != nil {
				t.Fatalf("expected suggestion report, got %v", err)
			}
			if report.Amount.Value != "121.00" || report.Amount.Evidence != "Total amount to pay: EUR 121.00" {
				t.Fatalf("unexpected selected amount: %+v", report.Amount)
			}
			if len(report.AgentContext.Candidates.Amount) != 1 || report.AgentContext.Candidates.Amount[0].Kind != amountCandidateKindPayableTotal {
				t.Fatalf("expected payable-total fallback, got %+v", report.AgentContext.Candidates.Amount)
			}
		})
	}
}

func TestSuggestPaymentDetailsFromTextUsesBarePayableTotalAmount(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Total amount to pay 15,00
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Amount != "15.00" {
		t.Fatalf("amount = %q, want 15.00", suggestion.Amount)
	}
}

func TestSuggestPaymentDetailsFromTextReportsAmbiguousPaymentInstructionAmounts(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{
			name: "separate lines",
			text: `
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Please pay EUR 86.36
Gelieve € 42,50 te betalen
Reference: INV-2026-001
`,
		},
		{
			name: "same line",
			text: `
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Please pay EUR 86.36 or pay 42,50 EUR
Reference: INV-2026-001
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := SuggestPaymentDetailsReportFromText(tt.text, PaymentDetails{})

			if err == nil {
				t.Fatalf("expected amount ambiguity")
			}
			if !strings.Contains(strings.ToLower(err.Error()), "amount") || !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
				t.Fatalf("expected amount ambiguity, got %v", err)
			}
			if report.Amount.Value != "" || len(report.AgentContext.Candidates.Amount) != 2 {
				t.Fatalf("expected ambiguous payment instruction candidates, report=%+v", report)
			}
		})
	}
}

func TestBuildSuggestedPaymentArtifactPlanOmitsPlanForAmbiguousPaymentInstructionAmounts(t *testing.T) {
	result, err := BuildSuggestedPaymentArtifactPlan(SuggestedPaymentArtifactPlanOptions{
		Text: `
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Please pay EUR 86.36
Gelieve € 42,50 te betalen
Reference: INV-2026-001
`,
		Output: QROutputOptions{Out: "invoice.qr", Format: "svg"},
	})

	if err == nil {
		t.Fatalf("expected amount ambiguity")
	}
	if !result.HasReport || result.HasPlan {
		t.Fatalf("expected report without plan, got %+v", result)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "amount") || !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Fatalf("expected amount ambiguity, got %v", err)
	}
}

func TestSuggestPaymentDetailsReportFromTextUsesSelectedAmountEvidence(t *testing.T) {
	report, err := SuggestPaymentDetailsReportFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Subtotal: EUR 42.50
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion report, got %v", err)
	}
	if report.Amount.Value != "42.50" {
		t.Fatalf("amount = %q, want 42.50", report.Amount.Value)
	}
	if report.Amount.Evidence != "Amount: EUR 42.50" {
		t.Fatalf("amount evidence = %q, want selected amount line", report.Amount.Evidence)
	}
}

func TestSuggestPaymentDetailsReportIncludesAgentContext(t *testing.T) {
	text := strings.Join([]string{
		"Invoice INV-2026-001",
		"Payee: ACME BV",
		"IBAN: BE68 5390 0754 7034",
		"IBAN duplicate BE68 5390 0754 7034",
		"Total amount to pay",
		"EUR 42.50",
		"Structured message +++123/4567/89002+++",
	}, "\n")

	report, err := SuggestPaymentDetailsReportFromText(text, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion report, got %v", err)
	}
	wantHash := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(text)))
	if report.AgentContext.SourceTextHash != wantHash {
		t.Fatalf("source hash = %q, want %q", report.AgentContext.SourceTextHash, wantHash)
	}
	assertObservedLine(t, report.AgentContext.ObservedLines, AgentContextObservedLine{
		Kind: "document_header",
		Line: 1,
		Text: "Invoice INV-2026-001",
	})
	assertObservedLine(t, report.AgentContext.ObservedLines, AgentContextObservedLine{
		Kind: "payee_context",
		Line: 2,
		Text: "Payee: ACME BV",
	})
	assertObservedLine(t, report.AgentContext.ObservedLines, AgentContextObservedLine{
		Kind: "iban_context",
		Line: 3,
		Text: "IBAN: BE68 5390 0754 7034",
	})
	assertObservedLine(t, report.AgentContext.ObservedLines, AgentContextObservedLine{
		Kind: "amount_context",
		Line: 5,
		Text: "Total amount to pay",
	})
	assertObservedLine(t, report.AgentContext.ObservedLines, AgentContextObservedLine{
		Kind: "reference_context",
		Line: 7,
		Text: "Structured message +++123/4567/89002+++",
	})

	ibanCandidates := report.AgentContext.Candidates.IBAN
	if len(ibanCandidates) != 1 {
		t.Fatalf("expected deduped IBAN candidate, got %+v", ibanCandidates)
	}
	if ibanCandidates[0].Value != "BE68 5390 0754 7034" || ibanCandidates[0].Normalized != "BE68539007547034" ||
		ibanCandidates[0].Evidence != "IBAN: BE68 5390 0754 7034" || ibanCandidates[0].Line != 3 {
		t.Fatalf("unexpected IBAN candidate: %+v", ibanCandidates[0])
	}
	amountCandidates := report.AgentContext.Candidates.Amount
	if len(amountCandidates) != 1 || amountCandidates[0].Value != "42.50" || amountCandidates[0].Normalized != "42.50" ||
		amountCandidates[0].Evidence != "EUR 42.50" || amountCandidates[0].Line != 6 {
		t.Fatalf("unexpected amount candidates: %+v", amountCandidates)
	}
	referenceCandidates := report.AgentContext.Candidates.Reference
	if len(referenceCandidates) != 1 || referenceCandidates[0].Value != "+++123/4567/89002+++" ||
		referenceCandidates[0].Kind != "structured" || referenceCandidates[0].Line != 7 {
		t.Fatalf("unexpected reference candidates: %+v", referenceCandidates)
	}
	if len(report.AgentContext.ReviewCandidates.Payee) != 0 ||
		len(report.AgentContext.ReviewCandidates.IBAN) != 0 ||
		len(report.AgentContext.ReviewCandidates.Amount) != 0 ||
		len(report.AgentContext.ReviewCandidates.Reference) != 0 {
		t.Fatalf("expected no review candidates for current behavior, got %+v", report.AgentContext.ReviewCandidates)
	}
}

func TestSuggestPaymentDetailsReportIncludesPaymentInstructionCompounds(t *testing.T) {
	text := strings.Join([]string{
		"Payee: ACME BV",
		"IBAN: BE68 5390 0754 7034",
		"Amount: EUR 42.50",
		"Reference: INV-2026-001",
		"Payable on receipt",
		"betalingsgegevens vindt u hieronder",
	}, "\n")

	report, err := SuggestPaymentDetailsReportFromText(text, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion report, got %v", err)
	}
	assertObservedLine(t, report.AgentContext.ObservedLines, AgentContextObservedLine{
		Kind: "payment_instruction",
		Line: 5,
		Text: "Payable on receipt",
	})
	assertObservedLine(t, report.AgentContext.ObservedLines, AgentContextObservedLine{
		Kind: "payment_instruction",
		Line: 6,
		Text: "betalingsgegevens vindt u hieronder",
	})
}

func TestSuggestionEvidenceSelectorKeepsReviewEvidenceOutOfSelectedDetails(t *testing.T) {
	selection := selectSuggestionEvidence([]suggestionEvidence{
		{
			Field:    "payee",
			Value:    "ACME BV",
			Evidence: "Payee: ACME BV",
			Line:     1,
			Status:   suggestionEvidenceStatusCandidate,
		},
		{
			Field:        "amount",
			Value:        "EUR 42.50",
			Normalized:   "42.50",
			Evidence:     "Total: EUR 42.50",
			Line:         2,
			Status:       suggestionEvidenceStatusReview,
			ReviewReason: "conflicting_generic_total",
		},
	}, PaymentDetails{})

	if selection.Details.Payee != "ACME BV" || selection.Payee.Evidence != "Payee: ACME BV" {
		t.Fatalf("unexpected selected payee: %+v", selection.Payee)
	}
	if selection.Details.Amount != "" || len(selection.Issues) != 3 {
		t.Fatalf("expected review amount not to satisfy required field, selection=%+v", selection)
	}
	if len(selection.ReviewCandidates.Amount) != 1 {
		t.Fatalf("expected amount review candidate, got %+v", selection.ReviewCandidates)
	}
	candidate := selection.ReviewCandidates.Amount[0]
	if candidate.Value != "EUR 42.50" || candidate.Normalized != "42.50" ||
		candidate.Evidence != "Total: EUR 42.50" || candidate.Line != 2 ||
		candidate.Reason != "conflicting_generic_total" {
		t.Fatalf("unexpected review candidate: %+v", candidate)
	}
}

func TestAgentContextPayeeCandidateCleansLabeledIBANLine(t *testing.T) {
	report, err := SuggestPaymentDetailsReportFromText(`
Payee: ACME BV - Main street 1 - IBAN BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion report, got %v", err)
	}
	payeeCandidates := report.AgentContext.Candidates.Payee
	if len(payeeCandidates) != 1 {
		t.Fatalf("expected one payee candidate, got %+v", payeeCandidates)
	}
	if payeeCandidates[0].Value != "ACME BV" {
		t.Fatalf("payee candidate = %q, want ACME BV", payeeCandidates[0].Value)
	}
}

func TestBuildSuggestedPaymentArtifactPlanReturnsReportAndPlan(t *testing.T) {
	result, err := BuildSuggestedPaymentArtifactPlan(SuggestedPaymentArtifactPlanOptions{
		Text: `
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`,
		Output: QROutputOptions{Out: "invoice.qr", Format: "svg"},
	})

	if err != nil {
		t.Fatalf("expected suggested payment artifact plan, got %v", err)
	}
	if !result.HasReport || !result.HasPlan {
		t.Fatalf("expected report and plan, got %+v", result)
	}
	if result.Report.IBAN.Source != "text" || result.Report.IBAN.Evidence != "IBAN: BE68 5390 0754 7034" {
		t.Fatalf("unexpected suggestion report: %+v", result.Report.IBAN)
	}
	if result.Plan.Details.IBAN != "BE68539007547034" || result.Plan.Output.Format != QRFormatSVG {
		t.Fatalf("unexpected payment artifact plan: %+v", result.Plan)
	}
	if result.Report.AgentContext.FullText != "" {
		t.Fatalf("expected compact Agent Context by default, got full text %q", result.Report.AgentContext.FullText)
	}
}

func TestBuildSuggestedPaymentArtifactPlanIncludesFullTextWhenRequested(t *testing.T) {
	text := `
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`
	result, err := BuildSuggestedPaymentArtifactPlan(SuggestedPaymentArtifactPlanOptions{
		Text:            text,
		Output:          QROutputOptions{Out: "invoice.qr", Format: "svg"},
		IncludeFullText: true,
	})

	if err != nil {
		t.Fatalf("expected suggested payment artifact plan, got %v", err)
	}
	if result.Report.AgentContext.FullText != text {
		t.Fatalf("full text = %q, want %q", result.Report.AgentContext.FullText, text)
	}
}

func TestBuildSuggestedPaymentArtifactPlanReturnsReportWithoutPlanForInvalidCompleteSuggestion(t *testing.T) {
	result, err := BuildSuggestedPaymentArtifactPlan(SuggestedPaymentArtifactPlanOptions{
		Text: `
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`,
		Overrides: PaymentDetails{Amount: "0"},
		Output:    QROutputOptions{Out: "invoice.qr", Format: "svg"},
	})

	if err == nil {
		t.Fatalf("expected invalid payment artifact plan")
	}
	if !result.HasReport || result.HasPlan {
		t.Fatalf("expected report without plan, got %+v", result)
	}
	if result.Report.Amount.Value != "0" || result.Report.Amount.Source != "override" {
		t.Fatalf("expected invalid override in report, got %+v", result.Report.Amount)
	}
	if !strings.Contains(err.Error(), "amount") {
		t.Fatalf("expected amount validation error, got %v", err)
	}
}

func TestBuildSuggestedPaymentArtifactPlanFailsClosedOnMalformedStructuredReference(t *testing.T) {
	result, err := BuildSuggestedPaymentArtifactPlan(SuggestedPaymentArtifactPlanOptions{
		Text: `
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Invoice: INV-2026-001
Payment message +++123/4567/89003+++
`,
		Output: QROutputOptions{Out: "invoice.qr", Format: "svg"},
	})

	if err == nil {
		t.Fatalf("expected malformed structured reference error")
	}
	if !result.HasReport || result.HasPlan {
		t.Fatalf("expected report without plan, got %+v", result)
	}
	if result.Report.Reference.Value != "+++123/4567/89003+++" {
		t.Fatalf("reference suggestion = %q, want malformed structured reference", result.Report.Reference.Value)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "reference") {
		t.Fatalf("expected reference validation error, got %v", err)
	}
}

func TestSuggestPaymentDetailsFromTextFindsPayeeFromCreditorIBANLine(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Total amount to pay

€ 15,00

Structured message
+++525/7350/37337+++

Mobile Vikings NV - Kempische steenweg 309 Bus 1 - 3500 Hasselt - BE 0886.946.917 - IBAN BE02 7370 2691 7240 - BIC KREDBEBB
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	assertSuggestedPaymentDetails(t, suggestion, PaymentDetails{
		Payee:     "Mobile Vikings NV",
		IBAN:      "BE02 7370 2691 7240",
		Amount:    "15.00",
		Reference: "+++525/7350/37337+++",
	})
}

func TestSuggestPaymentDetailsFromTextFindsDottedLegalSuffixPayeeFromCreditorIBANLine(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee B.V. - Main street 1 - IBAN BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Payee != "Payee B.V." {
		t.Fatalf("payee = %q, want Payee B.V.", suggestion.Payee)
	}
}

func TestSuggestPaymentDetailsFromTextPreservesHyphenatedPayeeFromCreditorIBANLine(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Smith-Jones BV - Main street 1 - IBAN BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Payee != "Smith-Jones BV" {
		t.Fatalf("payee = %q, want Smith-Jones BV", suggestion.Payee)
	}
}

func TestSuggestPaymentDetailsFromTextSkipsFooterPrefixBeforeCreditorIBANLine(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payment details - ACME BV - IBAN BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Payee != "ACME BV" {
		t.Fatalf("payee = %q, want ACME BV", suggestion.Payee)
	}
}

func TestSuggestPaymentDetailsFromTextHandlesCollapsedDashAfterCreditorIBANLine(t *testing.T) {
	for _, line := range []string{
		"ACME BV- Main street 1 - IBAN BE68 5390 0754 7034",
		"ACME BV–IBAN BE68 5390 0754 7034",
	} {
		t.Run(line, func(t *testing.T) {
			suggestion, err := SuggestPaymentDetailsFromText(`
`+line+`
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

			if err != nil {
				t.Fatalf("expected suggestion, got %v", err)
			}
			if suggestion.Payee != "ACME BV" {
				t.Fatalf("payee = %q, want ACME BV", suggestion.Payee)
			}
		})
	}
}

func TestSuggestPaymentDetailsFromTextFindsPayeeContainingIBANTextInCreditorIBANLine(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Ibanity BV - Main street 1 - IBAN BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Payee != "Ibanity BV" {
		t.Fatalf("payee = %q, want Ibanity BV", suggestion.Payee)
	}
}

func TestSuggestPaymentDetailsFromTextFindsPayeeContainingStandaloneIBANWordInCreditorIBANLine(t *testing.T) {
	for _, line := range []string{
		"IBAN Services BV - Main street 1 - IBAN BE68 5390 0754 7034",
		"Payee: IBAN Services BV - Main street 1 - IBAN BE68 5390 0754 7034",
	} {
		t.Run(line, func(t *testing.T) {
			suggestion, err := SuggestPaymentDetailsFromText(`
`+line+`
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

			if err != nil {
				t.Fatalf("expected suggestion, got %v", err)
			}
			if suggestion.Payee != "IBAN Services BV" {
				t.Fatalf("payee = %q, want IBAN Services BV", suggestion.Payee)
			}
		})
	}
}

func TestSuggestPaymentDetailsFromTextFindsPayeeBeforeUndashedAddressInCreditorIBANLine(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
ACME BV Main street 1 IBAN BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Payee != "ACME BV" {
		t.Fatalf("payee = %q, want ACME BV", suggestion.Payee)
	}
}

func TestSuggestPaymentDetailsFromTextStripsCreditorLabelFromIBANLine(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Creditor: ACME BV - Main street 1 - IBAN BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Payee != "ACME BV" {
		t.Fatalf("payee = %q, want ACME BV", suggestion.Payee)
	}
}

func TestSuggestPaymentDetailsFromTextCleansLabeledPayeeIBANLine(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV - Main street 1 - IBAN BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Payee != "ACME BV" {
		t.Fatalf("payee = %q, want ACME BV", suggestion.Payee)
	}
}

func TestSuggestPaymentDetailsFromTextPrefersExplicitPayeeOverInferredIBANLine(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
ACME Collections BV - Main street 1 - IBAN BE68 5390 0754 7034
Payee: ACME BV
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Payee != "ACME BV" {
		t.Fatalf("payee = %q, want ACME BV", suggestion.Payee)
	}
}

func TestSuggestPaymentDetailsFromTextFindsRepeatedFooterBrandPayee(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Invoice INV-2026-001
Customer
Jane Customer
Main street 1
Total amount to pay: EUR 42.50
Reference: INV-2026-001
ACME Energy CV
VAT BE 0123.456.789
info@acme.example
ACME Energy CV
IBAN BE68 5390 0754 7034
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Payee != "ACME Energy CV" {
		t.Fatalf("payee = %q, want ACME Energy CV", suggestion.Payee)
	}
}

func TestSuggestPaymentDetailsFromTextPrefersStrongPayeeOverFooterBrand(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Supplier: Billing Services BV
Total amount to pay: EUR 42.50
Reference: INV-2026-001
ACME Energy CV
VAT BE 0123.456.789
ACME Energy CV
IBAN BE68 5390 0754 7034
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Payee != "Billing Services BV" {
		t.Fatalf("payee = %q, want Billing Services BV", suggestion.Payee)
	}
}

func TestSuggestPaymentDetailsReportKeepsWeakFooterBrandAsReviewCandidate(t *testing.T) {
	report, err := SuggestPaymentDetailsReportFromText(`
Invoice INV-2026-001
Total amount to pay: EUR 42.50
Reference: INV-2026-001
ACME Energy CV
VAT BE 0123.456.789
IBAN BE68 5390 0754 7034
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected missing payee")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "payee") || !strings.Contains(strings.ToLower(err.Error()), "required") {
		t.Fatalf("expected payee required error, got %v", err)
	}
	if report.Payee.Value != "" {
		t.Fatalf("expected no selected payee, got %+v", report.Payee)
	}
	if len(report.AgentContext.Candidates.Payee) != 0 {
		t.Fatalf("expected no payee candidates, got %+v", report.AgentContext.Candidates.Payee)
	}
	if len(report.AgentContext.ReviewCandidates.Payee) != 1 {
		t.Fatalf("expected one payee review candidate, got %+v", report.AgentContext.ReviewCandidates.Payee)
	}
	candidate := report.AgentContext.ReviewCandidates.Payee[0]
	if candidate.Value != "ACME Energy CV" || candidate.Kind != "footer_brand" || candidate.Reason != "weak_footer_brand" {
		t.Fatalf("unexpected payee review candidate: %+v", candidate)
	}
}

func TestSuggestPaymentDetailsReportMarksDifferingFooterBrandPayeesAmbiguous(t *testing.T) {
	report, err := SuggestPaymentDetailsReportFromText(`
Invoice INV-2026-001
Total amount to pay: EUR 42.50
Reference: INV-2026-001
ACME Energy CV
VAT BE 0123.456.789
ACME Energy CV
info@acme.example
Billing Services BV
VAT BE 9876.543.210
Billing Services BV
IBAN BE68 5390 0754 7034
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected ambiguous payee")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "payee") || !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Fatalf("expected payee ambiguous error, got %v", err)
	}
	if report.Payee.Value != "" {
		t.Fatalf("expected no selected payee, got %+v", report.Payee)
	}
}

func TestSuggestPaymentDetailsReportDoesNotSelectCustomerNameHeaderAsPayee(t *testing.T) {
	report, err := SuggestPaymentDetailsReportFromText(`
Invoice INV-2026-001
Customer details
Name: Jane Customer BV
Address: Main street 1
Total amount to pay: EUR 42.50
Reference: INV-2026-001
IBAN BE68 5390 0754 7034
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected missing payee")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "payee") || !strings.Contains(strings.ToLower(err.Error()), "required") {
		t.Fatalf("expected payee required error, got %v", err)
	}
	if report.Payee.Value != "" {
		t.Fatalf("expected no selected payee, got %+v", report.Payee)
	}
	if len(report.AgentContext.Candidates.Payee) != 0 || len(report.AgentContext.ReviewCandidates.Payee) != 0 {
		t.Fatalf("expected no payee candidates for customer header, got candidates=%+v review=%+v", report.AgentContext.Candidates.Payee, report.AgentContext.ReviewCandidates.Payee)
	}
}

func TestSuggestPaymentDetailsReportDoesNotSelectRepeatedCustomerNameAsFooterBrandPayee(t *testing.T) {
	report, err := SuggestPaymentDetailsReportFromText(`
Invoice INV-2026-001
Customer details
Name: Jane Customer BV
VAT BE 0123.456.789
contact@customer.example
Delivery details
Name: Jane Customer BV
VAT BE 0123.456.789
Total amount to pay: EUR 42.50
Reference: INV-2026-001
IBAN BE68 5390 0754 7034
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected missing payee")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "payee") || !strings.Contains(strings.ToLower(err.Error()), "required") {
		t.Fatalf("expected payee required error, got %v", err)
	}
	if report.Payee.Value != "" {
		t.Fatalf("expected no selected payee, got %+v", report.Payee)
	}
	if len(report.AgentContext.Candidates.Payee) != 0 || len(report.AgentContext.ReviewCandidates.Payee) != 0 {
		t.Fatalf("expected no payee candidates for repeated customer name, got candidates=%+v review=%+v", report.AgentContext.Candidates.Payee, report.AgentContext.ReviewCandidates.Payee)
	}
}

func TestSuggestPaymentDetailsReportDoesNotSelectRepeatedCustomerLabelAsFooterBrandPayee(t *testing.T) {
	report, err := SuggestPaymentDetailsReportFromText(`
Invoice INV-2026-001
Billing details
Customer: Jane Customer BV
VAT BE 0123.456.789
contact@customer.example
Delivery details
Customer: Jane Customer BV
VAT BE 0123.456.789
Total amount to pay: EUR 42.50
Reference: INV-2026-001
IBAN BE68 5390 0754 7034
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected missing payee")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "payee") || !strings.Contains(strings.ToLower(err.Error()), "required") {
		t.Fatalf("expected payee required error, got %v", err)
	}
	if report.Payee.Value != "" {
		t.Fatalf("expected no selected payee, got %+v", report.Payee)
	}
	if len(report.AgentContext.Candidates.Payee) != 0 || len(report.AgentContext.ReviewCandidates.Payee) != 0 {
		t.Fatalf("expected no payee candidates for repeated customer label, got candidates=%+v review=%+v", report.AgentContext.Candidates.Payee, report.AgentContext.ReviewCandidates.Payee)
	}
}

func TestSuggestPaymentDetailsReportDoesNotSelectBodyCustomerBrandAsFooterBrandPayee(t *testing.T) {
	report, err := SuggestPaymentDetailsReportFromText(`
Invoice INV-2026-001
Customer details
Jane Customer BV
VAT BE 0123.456.789
contact@customer.example
Delivery details
Jane Customer BV
VAT BE 0123.456.789
Total amount to pay: EUR 42.50
Reference: INV-2026-001
IBAN BE68 5390 0754 7034
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected missing payee")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "payee") || !strings.Contains(strings.ToLower(err.Error()), "required") {
		t.Fatalf("expected payee required error, got %v", err)
	}
	if report.Payee.Value != "" {
		t.Fatalf("expected no selected payee, got %+v", report.Payee)
	}
	if len(report.AgentContext.Candidates.Payee) != 0 || len(report.AgentContext.ReviewCandidates.Payee) != 0 {
		t.Fatalf("expected no payee candidates for body customer brand, got candidates=%+v review=%+v", report.AgentContext.Candidates.Payee, report.AgentContext.ReviewCandidates.Payee)
	}
}

func TestSuggestPaymentDetailsReportDoesNotSelectCustomerNameLabelAsFooterBrandPayee(t *testing.T) {
	report, err := SuggestPaymentDetailsReportFromText(`
Invoice INV-2026-001
Total amount to pay: EUR 42.50
Reference: INV-2026-001
Customer Name: Jane Customer BV
VAT BE 0123.456.789
Customer Name: Jane Customer BV
IBAN BE68 5390 0754 7034
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected missing payee")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "payee") || !strings.Contains(strings.ToLower(err.Error()), "required") {
		t.Fatalf("expected payee required error, got %v", err)
	}
	if report.Payee.Value != "" {
		t.Fatalf("expected no selected payee, got %+v", report.Payee)
	}
	if len(report.AgentContext.Candidates.Payee) != 0 || len(report.AgentContext.ReviewCandidates.Payee) != 0 {
		t.Fatalf("expected no payee candidates for customer name label, got candidates=%+v review=%+v", report.AgentContext.Candidates.Payee, report.AgentContext.ReviewCandidates.Payee)
	}
}

func TestSuggestPaymentDetailsFromTextFindsPayNamedFooterBrandPayee(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Invoice INV-2026-001
Total amount to pay: EUR 42.50
Reference: INV-2026-001
Payment Services BV
VAT BE 0123.456.789
Payment Services BV
IBAN BE68 5390 0754 7034
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Payee != "Payment Services BV" {
		t.Fatalf("payee = %q, want Payment Services BV", suggestion.Payee)
	}
}

func TestBuildSuggestedPaymentArtifactPlanOmitsPlanForAmbiguousFooterBrandPayees(t *testing.T) {
	result, err := BuildSuggestedPaymentArtifactPlan(SuggestedPaymentArtifactPlanOptions{
		Text: `
Invoice INV-2026-001
Total amount to pay: EUR 42.50
Reference: INV-2026-001
ACME Energy CV
VAT BE 0123.456.789
ACME Energy CV
info@acme.example
Billing Services BV
VAT BE 9876.543.210
Billing Services BV
IBAN BE68 5390 0754 7034
`,
		Output: QROutputOptions{Out: "invoice.svg"},
	})

	if err == nil {
		t.Fatalf("expected ambiguous payee")
	}
	if !result.HasReport || result.HasPlan {
		t.Fatalf("expected report without plan, got %+v", result)
	}
}

func TestSuggestPaymentDetailsFromTextPrefersSplitTotalAmountToPay(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Total amount to pay

€ 15,00
Total

€ 0,00
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Amount != "15.00" {
		t.Fatalf("amount = %q, want 15.00", suggestion.Amount)
	}
}

func TestSuggestPaymentDetailsFromTextParsesSplitAmountWithNonBreakingCurrencySpace(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Total amount to pay
`+"\u00a0€\u00a015,00\u00a0"+`
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Amount != "15.00" {
		t.Fatalf("amount = %q, want 15.00", suggestion.Amount)
	}
}

func TestSuggestPaymentDetailsFromTextUsesInlinePreferredAmountAfterColon(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Total amount to pay: 15,00
Total: EUR 0,00
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Amount != "15.00" {
		t.Fatalf("amount = %q, want 15.00", suggestion.Amount)
	}
}

func TestSuggestPaymentDetailsFromTextDoesNotUseNextLineForGenericTotal(t *testing.T) {
	_, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Total

Reference: 42
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected amount error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "amount") {
		t.Fatalf("expected amount error, got %v", err)
	}
}

func TestSuggestPaymentDetailsFromTextDoesNotUseReferenceAsSplitPayableAmount(t *testing.T) {
	_, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Total amount to pay
Reference: 42
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected amount error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "amount") {
		t.Fatalf("expected amount error, got %v", err)
	}
}

func TestSuggestPaymentDetailsFromTextFallsBackAfterNonAmountSplitLine(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Total amount to pay
Due date: 2026-06-30
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Amount != "42.50" {
		t.Fatalf("amount = %q, want 42.50", suggestion.Amount)
	}
}

func TestSuggestPaymentDetailsFromTextPrefersSplitCurrencyOverLabelNumbers(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount to pay within 7 days
€ 15,00
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Amount != "15.00" {
		t.Fatalf("amount = %q, want 15.00", suggestion.Amount)
	}
}

func TestSuggestPaymentDetailsFromTextDoesNotUseColonLabelTextAsPreferredAmount(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount to pay: within 7 days
€ 15,00
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Amount != "15.00" {
		t.Fatalf("amount = %q, want 15.00", suggestion.Amount)
	}
}

func TestSuggestPaymentDetailsFromTextDoesNotUseLabeledVATAsSplitPayableAmount(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Total amount to pay
VAT: EUR 5,00
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Amount != "42.50" {
		t.Fatalf("amount = %q, want 42.50", suggestion.Amount)
	}
}

func TestSuggestPaymentDetailsFromTextReportsAmbiguousDateAndAmountWithoutCurrency(t *testing.T) {
	_, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Total due by 2026-06-30: 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected ambiguity error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "amount") || !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Fatalf("expected amount ambiguity error, got %v", err)
	}
}

func TestSuggestPaymentDetailsFromTextDoesNotTruncateMalformedThousandsAmount(t *testing.T) {
	_, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 1.234,567
Reference: INV-2026-001
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected amount error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "amount") {
		t.Fatalf("expected amount error, got %v", err)
	}
}

func TestSuggestPaymentDetailsFromTextRejectsMalformedSpaceGroupedAmounts(t *testing.T) {
	for _, amount := range []string{"1 23,45", "12 34,56"} {
		t.Run(amount, func(t *testing.T) {
			_, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR `+amount+`
Reference: INV-2026-001
`, PaymentDetails{})

			if err == nil {
				t.Fatalf("expected amount error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), "amount") {
				t.Fatalf("expected amount error, got %v", err)
			}
		})
	}
}

func TestSuggestPaymentDetailsFromTextRejectsSignedNegativeAmounts(t *testing.T) {
	for _, line := range []string{"Amount: EUR -42.50", "Amount: -42.50", "Amount: (42.50)", "Amount: EUR (42.50)"} {
		t.Run(line, func(t *testing.T) {
			_, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
`+line+`
Reference: INV-2026-001
`, PaymentDetails{})

			if err == nil {
				t.Fatalf("expected amount error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), "amount") {
				t.Fatalf("expected amount error, got %v", err)
			}
		})
	}
}

func TestSuggestPaymentDetailsFromTextReportsMissingFields(t *testing.T) {
	_, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected missing field error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "iban") {
		t.Fatalf("expected iban error, got %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "required") {
		t.Fatalf("expected required field wording, got %v", err)
	}
}

func TestSuggestPaymentDetailsFromTextReportsAmbiguousIBANs(t *testing.T) {
	_, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Alternative IBAN: NL91 ABNA 0417 1643 00
Amount: EUR 42.50
Reference: INV-2026-001
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected ambiguity error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "iban") || !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Fatalf("expected iban ambiguity error, got %v", err)
	}
}

func TestSuggestPaymentDetailsFromTextReportsAmbiguousAmounts(t *testing.T) {
	_, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Total: EUR 12.00
Reference: INV-2026-001
`, PaymentDetails{})

	if err == nil {
		t.Fatalf("expected ambiguity error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "amount") || !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Fatalf("expected amount ambiguity error, got %v", err)
	}
}

func TestSuggestPaymentDetailsFromTextFindsBelgianStructuredReference(t *testing.T) {
	suggestion, err := SuggestPaymentDetailsFromText(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
+++123/4567/89002+++
`, PaymentDetails{})

	if err != nil {
		t.Fatalf("expected suggestion, got %v", err)
	}
	if suggestion.Reference != "+++123/4567/89002+++" {
		t.Fatalf("reference = %q", suggestion.Reference)
	}
}

func assertSuggestedPaymentDetails(t *testing.T, got SuggestedPaymentDetails, want PaymentDetails) {
	t.Helper()

	if got.Payee != want.Payee {
		t.Fatalf("payee = %q, want %q", got.Payee, want.Payee)
	}
	if got.IBAN != want.IBAN {
		t.Fatalf("iban = %q, want %q", got.IBAN, want.IBAN)
	}
	if got.Amount != want.Amount {
		t.Fatalf("amount = %q, want %q", got.Amount, want.Amount)
	}
	if got.Reference != want.Reference {
		t.Fatalf("reference = %q, want %q", got.Reference, want.Reference)
	}
}

func assertObservedLine(t *testing.T, lines []AgentContextObservedLine, want AgentContextObservedLine) {
	t.Helper()

	for _, line := range lines {
		if line == want {
			return
		}
	}
	t.Fatalf("missing observed line %+v in %+v", want, lines)
}

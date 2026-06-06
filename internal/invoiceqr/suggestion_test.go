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

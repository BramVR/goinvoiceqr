package invoiceqr

import (
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

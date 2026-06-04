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

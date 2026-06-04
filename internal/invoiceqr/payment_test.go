package invoiceqr

import (
	"strings"
	"testing"
)

func TestValidatePaymentDetailsNormalizesFields(t *testing.T) {
	details, err := ValidatePaymentDetails(PaymentDetails{
		Payee:     "  ACME BV  ",
		IBAN:      " be68 5390 0754 7034 ",
		Amount:    "42.5",
		Reference: "+++123/4567/89002+++",
	})

	if err != nil {
		t.Fatalf("expected valid payment details, got %v", err)
	}
	if details.Payee != "ACME BV" {
		t.Fatalf("payee = %q", details.Payee)
	}
	if details.IBAN != "BE68539007547034" {
		t.Fatalf("iban = %q", details.IBAN)
	}
	if details.Amount != "42.50" {
		t.Fatalf("amount = %q", details.Amount)
	}
	if details.Reference.Kind != StructuredReference {
		t.Fatalf("reference kind = %v", details.Reference.Kind)
	}
	if details.Reference.Value != "+++123/4567/89002+++" {
		t.Fatalf("reference value = %q", details.Reference.Value)
	}
}

func TestValidatePaymentDetailsAcceptsDocumentedStructuredReferenceSyntax(t *testing.T) {
	details, err := ValidatePaymentDetails(PaymentDetails{
		Payee:     "ACME BV",
		IBAN:      "BE68539007547034",
		Amount:    "42.50",
		Reference: "+++/123/4567/89002+++",
	})

	if err != nil {
		t.Fatalf("expected documented structured reference syntax to pass, got %v", err)
	}
	if details.Reference.Kind != StructuredReference {
		t.Fatalf("reference kind = %v", details.Reference.Kind)
	}
}

func TestValidatePaymentDetailsAcceptsSEPAIBANAndUnstructuredReference(t *testing.T) {
	details, err := ValidatePaymentDetails(PaymentDetails{
		Payee:     "Dutch Supplier",
		IBAN:      "NL91 ABNA 0417 1643 00",
		Amount:    "12",
		Reference: "Invoice 2026-001",
		BIC:       "gebabebb",
	})

	if err != nil {
		t.Fatalf("expected valid payment details, got %v", err)
	}
	if details.IBAN != "NL91ABNA0417164300" {
		t.Fatalf("iban = %q", details.IBAN)
	}
	if details.Amount != "12.00" {
		t.Fatalf("amount = %q", details.Amount)
	}
	if details.Reference.Kind != UnstructuredReference {
		t.Fatalf("reference kind = %v", details.Reference.Kind)
	}
	if details.BIC != "GEBABEBB" {
		t.Fatalf("bic = %q", details.BIC)
	}
}

func TestValidatePaymentDetailsCountsTextLimitsByCharacters(t *testing.T) {
	payee := strings.Repeat("é", 70)
	reference := strings.Repeat("é", 140)

	details, err := ValidatePaymentDetails(PaymentDetails{
		Payee:     payee,
		IBAN:      "BE68539007547034",
		Amount:    "1.00",
		Reference: reference,
	})

	if err != nil {
		t.Fatalf("expected utf-8 text within character limits to pass, got %v", err)
	}
	if details.Payee != payee {
		t.Fatalf("payee = %q", details.Payee)
	}
	if details.Reference.Value != reference {
		t.Fatalf("reference = %q", details.Reference.Value)
	}
}

func TestValidatePaymentDetailsRejectsFieldSpecificFailures(t *testing.T) {
	tests := []struct {
		name     string
		details  PaymentDetails
		contains string
	}{
		{
			name:     "invalid iban checksum",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547035", Amount: "1.00", Reference: "INV-1"},
			contains: "iban",
		},
		{
			name:     "invalid iban characters",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68_39007547034", Amount: "1.00", Reference: "INV-1"},
			contains: "iban",
		},
		{
			name:     "invalid iban unicode digit",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68٥39007547034", Amount: "1.00", Reference: "INV-1"},
			contains: "iban",
		},
		{
			name:     "iban check digits must be digits",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BEOX539007547034", Amount: "1.00", Reference: "INV-1"},
			contains: "iban",
		},
		{
			name:     "unknown iban country",
			details:  PaymentDetails{Payee: "ACME", IBAN: "ZZ66000000000000", Amount: "1.00", Reference: "INV-1"},
			contains: "iban",
		},
		{
			name:     "non sepa iban country",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BR9700360305000010009795493P1", Amount: "1.00", Reference: "INV-1"},
			contains: "iban",
		},
		{
			name:     "over precise amount",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "1.234", Reference: "INV-1"},
			contains: "amount",
		},
		{
			name:     "unicode zero amount",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "٠", Reference: "INV-1"},
			contains: "amount",
		},
		{
			name:     "negative amount",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "-1.00", Reference: "INV-1"},
			contains: "amount",
		},
		{
			name:     "malformed amount",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "1..00", Reference: "INV-1"},
			contains: "amount",
		},
		{
			name:     "dangling decimal amount",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "42.", Reference: "INV-1"},
			contains: "amount",
		},
		{
			name:     "dangling comma amount",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "42,", Reference: "INV-1"},
			contains: "amount",
		},
		{
			name:     "zero amount",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "0", Reference: "INV-1"},
			contains: "amount",
		},
		{
			name:     "invalid structured reference checksum",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "1.00", Reference: "+++/123/4567/89003+++"},
			contains: "reference",
		},
		{
			name:     "invalid bic length",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "1.00", Reference: "INV-1", BIC: "GEBABEB"},
			contains: "bic",
		},
		{
			name:     "invalid bic separator",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "1.00", Reference: "INV-1", BIC: "GEBABEBB XXX"},
			contains: "bic",
		},
		{
			name:     "invalid bic unicode digit",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "1.00", Reference: "INV-1", BIC: "GEBABE١"},
			contains: "bic",
		},
		{
			name:     "payee over character limit",
			details:  PaymentDetails{Payee: strings.Repeat("A", 71), IBAN: "BE68539007547034", Amount: "1.00", Reference: "INV-1"},
			contains: "payee",
		},
		{
			name:     "payee line break",
			details:  PaymentDetails{Payee: "ACME\nBV", IBAN: "BE68539007547034", Amount: "1.00", Reference: "INV-1"},
			contains: "payee",
		},
		{
			name:     "reference over character limit",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "1.00", Reference: strings.Repeat("A", 141)},
			contains: "reference",
		},
		{
			name:     "reference line break",
			details:  PaymentDetails{Payee: "ACME", IBAN: "BE68539007547034", Amount: "1.00", Reference: "INV-1\nnext"},
			contains: "reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePaymentDetails(tt.details)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), tt.contains) {
				t.Fatalf("expected %q in %v", tt.contains, err)
			}
		})
	}
}

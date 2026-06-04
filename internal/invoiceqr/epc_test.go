package invoiceqr

import (
	"strings"
	"testing"
)

func TestBuildEPCPayloadWithStructuredReferenceAndEmptyBIC(t *testing.T) {
	payload, err := BuildEPCPayload(ConfirmedPaymentDetails{
		Payee:  "ACME BV",
		IBAN:   "BE68539007547034",
		Amount: "42.50",
		Reference: RemittanceReference{
			Kind:  StructuredReference,
			Value: "+++123/4567/89002+++",
		},
	})

	if err != nil {
		t.Fatalf("expected payload, got %v", err)
	}

	want := strings.Join([]string{
		"BCD",
		"002",
		"1",
		"SCT",
		"",
		"ACME BV",
		"BE68539007547034",
		"EUR42.50",
		"",
		"+++123/4567/89002+++",
		"",
		"",
	}, "\n")
	if payload != want {
		t.Fatalf("payload mismatch\nwant:\n%q\ngot:\n%q", want, payload)
	}
}

func TestBuildEPCPayloadWithUnstructuredReferenceAndBIC(t *testing.T) {
	payload, err := BuildEPCPayload(ConfirmedPaymentDetails{
		Payee:  "Dutch Supplier",
		IBAN:   "NL91ABNA0417164300",
		Amount: "12.00",
		Reference: RemittanceReference{
			Kind:  UnstructuredReference,
			Value: "Invoice 2026-001",
		},
		BIC: "ABNANL2A",
	})

	if err != nil {
		t.Fatalf("expected payload, got %v", err)
	}

	want := strings.Join([]string{
		"BCD",
		"002",
		"1",
		"SCT",
		"ABNANL2A",
		"Dutch Supplier",
		"NL91ABNA0417164300",
		"EUR12.00",
		"",
		"",
		"Invoice 2026-001",
		"",
	}, "\n")
	if payload != want {
		t.Fatalf("payload mismatch\nwant:\n%q\ngot:\n%q", want, payload)
	}
}

func TestBuildEPCPayloadRejectsUnknownRemittanceKind(t *testing.T) {
	_, err := BuildEPCPayload(ConfirmedPaymentDetails{
		Payee:  "ACME BV",
		IBAN:   "BE68539007547034",
		Amount: "42.50",
		Reference: RemittanceReference{
			Kind:  RemittanceKind("unknown"),
			Value: "INV-1",
		},
	})

	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "reference") {
		t.Fatalf("expected reference error, got %v", err)
	}
}

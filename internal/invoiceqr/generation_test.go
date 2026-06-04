package invoiceqr

import (
	"errors"
	"strings"
	"testing"
)

func TestGeneratePaymentArtifactValidatesBeforeConfirmationAndOutput(t *testing.T) {
	confirmCalled := false
	writeCalled := false

	err := generatePaymentArtifact(
		PaymentGenerationOptions{
			Details: PaymentDetails{
				Payee:     "ACME BV",
				IBAN:      "BE68539007547035",
				Amount:    "42.50",
				Reference: "INV-1",
			},
			Output: QROutputOptions{Out: "invoice.svg"},
		},
		func(ValidatedPaymentDetails) (bool, error) {
			confirmCalled = true
			return true, nil
		},
		func(string, QROutputOptions) error {
			writeCalled = true
			return nil
		},
	)

	if err == nil {
		t.Fatalf("expected validation error")
	}
	if confirmCalled {
		t.Fatalf("expected validation before confirmation")
	}
	if writeCalled {
		t.Fatalf("expected no output write after validation failure")
	}
}

func TestGeneratePaymentArtifactRefusalStopsBeforeOutput(t *testing.T) {
	writeCalled := false

	err := generatePaymentArtifact(
		PaymentGenerationOptions{
			Details: validManualPaymentDetails(),
			Output:  QROutputOptions{Out: "invoice.svg"},
		},
		func(details ValidatedPaymentDetails) (bool, error) {
			if details.Payee != "ACME BV" {
				t.Fatalf("expected normalized details, got %q", details.Payee)
			}
			return false, nil
		},
		func(string, QROutputOptions) error {
			writeCalled = true
			return nil
		},
	)

	if err == nil {
		t.Fatalf("expected refusal error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "confirmation") {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	if writeCalled {
		t.Fatalf("expected no output write after refusal")
	}
}

func TestGeneratePaymentArtifactYesSkipsConfirmationAndWrites(t *testing.T) {
	var gotPayload string
	var gotOutput QROutputOptions

	err := generatePaymentArtifact(
		PaymentGenerationOptions{
			Details:          validManualPaymentDetails(),
			Output:           QROutputOptions{Out: "invoice.qr", Format: "svg", Force: true},
			SkipConfirmation: true,
		},
		func(ValidatedPaymentDetails) (bool, error) {
			t.Fatalf("expected --yes to skip confirmation")
			return false, nil
		},
		func(payload string, output QROutputOptions) error {
			gotPayload = payload
			gotOutput = output
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected generation, got %v", err)
	}
	for _, want := range []string{"BCD\n002\n1\nSCT", "ACME BV", "BE68539007547034", "EUR42.50", "INV-1"} {
		if !strings.Contains(gotPayload, want) {
			t.Fatalf("expected payload to contain %q, got %q", want, gotPayload)
		}
	}
	if gotOutput.Out != "invoice.qr" || gotOutput.Format != "svg" || !gotOutput.Force {
		t.Fatalf("unexpected output options: %+v", gotOutput)
	}
}

func TestGeneratePaymentArtifactOrdersConfirmationBeforeOutput(t *testing.T) {
	events := []string{}

	err := generatePaymentArtifact(
		PaymentGenerationOptions{
			Details: validManualPaymentDetails(),
			Output:  QROutputOptions{Out: "invoice.svg"},
		},
		func(ValidatedPaymentDetails) (bool, error) {
			events = append(events, "confirm")
			return true, nil
		},
		func(string, QROutputOptions) error {
			events = append(events, "write")
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected generation, got %v", err)
	}
	if strings.Join(events, ",") != "confirm,write" {
		t.Fatalf("unexpected event order: %v", events)
	}
}

func TestGeneratePaymentArtifactPropagatesOutputError(t *testing.T) {
	writeErr := errors.New("write failed")

	err := generatePaymentArtifact(
		PaymentGenerationOptions{
			Details:          validManualPaymentDetails(),
			Output:           QROutputOptions{Out: "invoice.svg"},
			SkipConfirmation: true,
		},
		nil,
		func(string, QROutputOptions) error {
			return writeErr
		},
	)

	if !errors.Is(err, writeErr) {
		t.Fatalf("expected write error, got %v", err)
	}
}

func validManualPaymentDetails() PaymentDetails {
	return PaymentDetails{
		Payee:     " ACME BV ",
		IBAN:      "be68 5390 0754 7034",
		Amount:    "42.5",
		Reference: "INV-1",
	}
}

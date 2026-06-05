package invoiceqr

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPaymentArtifactPlanValidatesPayloadAndPreflightsOutputWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "invoice.svg")

	plan, err := BuildPaymentArtifactPlan(PaymentArtifactPlanOptions{
		Details: validManualPaymentDetails(),
		Output:  QROutputOptions{Out: " " + out + " "},
	})

	if err != nil {
		t.Fatalf("expected plan, got %v", err)
	}
	if plan.Details.Payee != "ACME BV" || plan.Details.IBAN != "BE68539007547034" || plan.Details.Amount != "42.50" {
		t.Fatalf("unexpected normalized details: %+v", plan.Details)
	}
	if plan.ReferenceKind != UnstructuredReference {
		t.Fatalf("expected unstructured reference kind, got %q", plan.ReferenceKind)
	}
	if plan.EPC.ServiceTag != "BCD" || plan.EPC.Version != "002" || plan.EPC.Currency != "EUR" {
		t.Fatalf("unexpected EPC metadata: %+v", plan.EPC)
	}
	for _, want := range []string{"BCD\n002\n1\nSCT", "ACME BV", "BE68539007547034", "EUR42.50", "INV-1"} {
		if !strings.Contains(plan.EPC.Payload, want) {
			t.Fatalf("expected payload to contain %q, got %q", want, plan.EPC.Payload)
		}
	}
	if plan.Output.Path != out || plan.Output.Format != QRFormatSVG || plan.Output.Exists || plan.Output.WillOverwrite {
		t.Fatalf("unexpected output preflight: %+v", plan.Output)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no output file, got stat error %v", err)
	}
}

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
		func(string, QROutputPreflight) error {
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
		func(string, QROutputPreflight) error {
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

func TestGeneratePaymentArtifactPreflightsOutputBeforeConfirmation(t *testing.T) {
	confirmCalled := false

	err := GeneratePaymentArtifact(
		PaymentGenerationOptions{
			Details: validManualPaymentDetails(),
			Output:  QROutputOptions{Out: "invoice.unknown"},
		},
		func(ValidatedPaymentDetails) (bool, error) {
			confirmCalled = true
			return true, nil
		},
	)

	if err == nil {
		t.Fatalf("expected output preflight error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "format") {
		t.Fatalf("expected format error, got %v", err)
	}
	if confirmCalled {
		t.Fatalf("expected output preflight before confirmation")
	}
}

func TestGeneratePaymentArtifactPreflightsExistingOutputBeforeConfirmation(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "invoice.svg")
	if err := os.WriteFile(out, []byte("existing"), 0o644); err != nil {
		t.Fatalf("seed output: %v", err)
	}
	confirmCalled := false

	err := GeneratePaymentArtifact(
		PaymentGenerationOptions{
			Details: validManualPaymentDetails(),
			Output:  QROutputOptions{Out: out},
		},
		func(ValidatedPaymentDetails) (bool, error) {
			confirmCalled = true
			return true, nil
		},
	)

	if err == nil {
		t.Fatalf("expected existing output error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
		t.Fatalf("expected existing output error, got %v", err)
	}
	if confirmCalled {
		t.Fatalf("expected output preflight before confirmation")
	}
}

func TestGeneratePaymentArtifactPreflightsForceSymlinkBeforeConfirmation(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.svg")
	out := filepath.Join(dir, "invoice.svg")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	symlinkOrSkip(t, target, out)
	confirmCalled := false

	err := GeneratePaymentArtifact(
		PaymentGenerationOptions{
			Details: validManualPaymentDetails(),
			Output:  QROutputOptions{Out: out, Force: true},
		},
		func(ValidatedPaymentDetails) (bool, error) {
			confirmCalled = true
			return true, nil
		},
	)

	if err == nil {
		t.Fatalf("expected symlink output error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
	if confirmCalled {
		t.Fatalf("expected output preflight before confirmation")
	}
}

func TestGeneratePaymentArtifactYesSkipsConfirmationAndWrites(t *testing.T) {
	var gotPayload string
	var gotOutput QROutputPreflight

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
		func(payload string, output QROutputPreflight) error {
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
	if gotOutput.Path != "invoice.qr" || gotOutput.Format != QRFormatSVG || !gotOutput.Force {
		t.Fatalf("unexpected output options: %+v", gotOutput)
	}
}

func TestGeneratePaymentArtifactWritesFromPlannedOutput(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "invoice.svg")
	if err := os.WriteFile(out, []byte("existing"), 0o644); err != nil {
		t.Fatalf("seed output: %v", err)
	}
	var gotOutput QROutputPreflight

	err := generatePaymentArtifact(
		PaymentGenerationOptions{
			Details:          validManualPaymentDetails(),
			Output:           QROutputOptions{Out: out, Force: true},
			SkipConfirmation: true,
		},
		nil,
		func(_ string, output QROutputPreflight) error {
			gotOutput = output
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected generation, got %v", err)
	}
	if gotOutput.Path != out || gotOutput.Format != QRFormatSVG || !gotOutput.Exists || !gotOutput.WillOverwrite {
		t.Fatalf("expected planned overwrite output, got %+v", gotOutput)
	}
}

func TestGeneratePaymentArtifactWithResultReturnsPlanAndArtifactMetadata(t *testing.T) {
	result, err := generatePaymentArtifactWithResult(
		PaymentGenerationOptions{
			Details:          validManualPaymentDetails(),
			Output:           QROutputOptions{Out: "invoice.qr", Format: "svg"},
			SkipConfirmation: true,
		},
		nil,
		func(payload string, output QROutputPreflight) (QRArtifactWriteResult, error) {
			if !strings.Contains(payload, "ACME BV") {
				t.Fatalf("expected payload to contain payee, got %q", payload)
			}
			return QRArtifactWriteResult{
				Path:      output.Path,
				Format:    output.Format,
				ByteCount: 123,
			}, nil
		},
	)

	if err != nil {
		t.Fatalf("expected generation result, got %v", err)
	}
	if result.Plan.Details.Payee != "ACME BV" || result.Plan.ReferenceKind != UnstructuredReference {
		t.Fatalf("unexpected plan: %+v", result.Plan)
	}
	if result.Artifact.Path != "invoice.qr" || result.Artifact.Format != QRFormatSVG || result.Artifact.ByteCount != 123 {
		t.Fatalf("unexpected artifact metadata: %+v", result.Artifact)
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
		func(string, QROutputPreflight) error {
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
		func(string, QROutputPreflight) error {
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

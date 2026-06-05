package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bramvr/goinvoiceqr/internal/invoiceqr"
)

func TestCommandHelpExposesPaymentDetailsFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		flags     []string
		helpTexts []string
	}{
		{
			name: "generate",
			args: []string{"generate", "--help"},
			flags: []string{
				"--payee=STRING",
				"--iban=STRING",
				"--amount=STRING",
				"--reference=STRING",
				"--bic=STRING",
				"--out=STRING",
				"--format=STRING",
				"--force",
				"--yes",
				"--dry-run",
				"--json",
			},
		},
		{
			name: "validate",
			args: []string{"validate", "--help"},
			flags: []string{
				"--payee=STRING",
				"--iban=STRING",
				"--amount=STRING",
				"--reference=STRING",
				"--bic=STRING",
			},
		},
		{
			name: "from-text",
			args: []string{"from-text", "--help"},
			flags: []string{
				"--payee=STRING",
				"--iban=STRING",
				"--amount=STRING",
				"--reference=STRING",
				"--bic=STRING",
				"--out=STRING",
				"--format=STRING",
				"--force",
				"--dry-run",
				"--json",
			},
			helpTexts: []string{
				"Requires --json.",
				"Requires --dry-run.",
			},
		},
		{
			name: "from-pdf",
			args: []string{"from-pdf", "--help"},
			flags: []string{
				"--payee=STRING",
				"--iban=STRING",
				"--amount=STRING",
				"--reference=STRING",
				"--bic=STRING",
				"--out=STRING",
				"--format=STRING",
				"--force",
				"--dry-run",
				"--json",
			},
			helpTexts: []string{
				"Requires --json.",
				"Requires --dry-run.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("go", append([]string{"run", "."}, tt.args...)...)

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("expected help to succeed, output:\n%s", output)
			}
			for _, flag := range tt.flags {
				if !strings.Contains(string(output), flag) {
					t.Fatalf("expected help to contain %q, got:\n%s", flag, output)
				}
			}
			normalizedHelp := strings.Join(strings.Fields(string(output)), " ")
			for _, helpText := range tt.helpTexts {
				if !strings.Contains(normalizedHelp, helpText) {
					t.Fatalf("expected help to contain %q, got:\n%s", helpText, output)
				}
			}
		})
	}
}

func TestValidatePrintsNormalizedPaymentDetails(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "validate",
		"--payee", " ACME BV ",
		"--iban", "be68 5390 0754 7034",
		"--amount", "42.5",
		"--reference", "INV-2026-001",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected validate to succeed, output:\n%s", output)
	}

	for _, want := range []string{
		"Payment Details",
		"Payee: ACME BV",
		"IBAN: BE68539007547034",
		"Amount: EUR42.50",
		"Reference: INV-2026-001",
		"Reference Type: Unstructured Remittance Information",
	} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestValidateJSONPrintsNormalizedPaymentDetailsEnvelope(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "validate",
		"--payee", " ACME BV ",
		"--iban", "be68 5390 0754 7034",
		"--amount", "42.5",
		"--reference", "INV-2026-001",
		"--bic", "gebabebb",
		"--json",
	)

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected validate JSON to succeed, output:\n%s", output)
	}
	if strings.Contains(string(output), "Payment Details") {
		t.Fatalf("expected JSON-only stdout, got:\n%s", output)
	}
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Payee     string `json:"payee"`
			IBAN      string `json:"iban"`
			Amount    string `json:"amount"`
			BIC       string `json:"bic"`
			Reference struct {
				Value string `json:"value"`
				Kind  string `json:"kind"`
			} `json:"reference"`
		} `json:"data"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("expected valid JSON, got %v:\n%s", err, output)
	}
	if !envelope.Success || envelope.Error != nil {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if envelope.Data.Payee != "ACME BV" || envelope.Data.IBAN != "BE68539007547034" || envelope.Data.Amount != "42.50" {
		t.Fatalf("unexpected normalized data: %+v", envelope.Data)
	}
	if envelope.Data.BIC != "GEBABEBB" {
		t.Fatalf("unexpected BIC data: %+v", envelope.Data)
	}
	if envelope.Data.Reference.Value != "INV-2026-001" || envelope.Data.Reference.Kind != "unstructured" {
		t.Fatalf("unexpected reference data: %+v", envelope.Data.Reference)
	}
}

func TestValidateJSONPrintsFieldErrorEnvelope(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	cmd := exec.Command(binary, "validate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547035",
		"--amount", "42.50",
		"--reference", "INV-2026-001",
		"--json",
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected validate JSON to fail")
	}
	if strings.Contains(stdout.String(), "Payment Details") {
		t.Fatalf("expected JSON-only stdout, got:\n%s", stdout.String())
	}
	var envelope struct {
		Success bool `json:"success"`
		Data    any  `json:"data"`
		Error   struct {
			Code    string `json:"code"`
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid JSON stdout, got %v:\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("expected empty stderr in JSON mode, got:\n%s", stderr.String())
	}
	if envelope.Success || envelope.Data != nil {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
	if envelope.Error.Code != "validation_error" || envelope.Error.Field != "iban" || !strings.Contains(envelope.Error.Message, "invalid checksum") {
		t.Fatalf("unexpected error data: %+v", envelope.Error)
	}
}

func TestValidateJSONOmitsEmptyBIC(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "validate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547034",
		"--amount", "42.50",
		"--reference", "INV-2026-001",
		"--json",
	)

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected validate JSON to succeed, output:\n%s", output)
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("expected valid JSON, got %v:\n%s", err, output)
	}
	if _, ok := envelope.Data["bic"]; ok {
		t.Fatalf("expected empty BIC to be omitted, got:\n%s", output)
	}
}

func TestValidatePrintsFieldSpecificErrors(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "validate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547035",
		"--amount", "42.50",
		"--reference", "INV-2026-001",
	)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected validate to fail, output:\n%s", output)
	}
	if !strings.Contains(strings.ToLower(string(output)), "iban") {
		t.Fatalf("expected field-specific iban error, got:\n%s", output)
	}
}

func TestGenerateYesWritesQRArtifact(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command("go", "run", ".", "generate",
		"--payee", " ACME BV ",
		"--iban", "be68 5390 0754 7034",
		"--amount", "42.5",
		"--reference", "INV-1",
		"--out", out,
		"--yes",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected generate to succeed, output:\n%s", output)
	}
	assertSVGOutput(t, out)
	if strings.Contains(string(output), "Write QR artifact") {
		t.Fatalf("expected --yes to skip confirmation, got:\n%s", output)
	}
}

func TestGenerateDryRunJSONPrintsPaymentArtifactPlanWithoutPromptOrWrite(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command("go", "run", ".", "generate",
		"--payee", " ACME BV ",
		"--iban", "be68 5390 0754 7034",
		"--amount", "42.5",
		"--reference", "INV-1",
		"--out", out,
		"--dry-run",
		"--json",
	)
	cmd.Stdin = strings.NewReader("no\n")

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected dry-run generate JSON to succeed, output:\n%s", output)
	}
	if strings.Contains(string(output), "Write QR artifact") || strings.Contains(string(output), "Payment Details") {
		t.Fatalf("expected JSON-only dry-run output, got:\n%s", output)
	}
	if strings.Contains(strings.ToLower(string(output)), "confidence") {
		t.Fatalf("expected no AI confidence scores, got:\n%s", output)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no dry-run output file, got stat err %v", statErr)
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			PaymentDetails paymentDetailsJSON `json:"payment_details"`
			EPC            struct {
				ServiceTag     string `json:"service_tag"`
				Version        string `json:"version"`
				CharacterSet   string `json:"character_set"`
				Identification string `json:"identification"`
				Currency       string `json:"currency"`
				Payload        string `json:"payload"`
			} `json:"epc"`
			Output struct {
				Path          string `json:"path"`
				Format        string `json:"format"`
				Force         bool   `json:"force"`
				Exists        bool   `json:"exists"`
				IsSymlink     bool   `json:"is_symlink"`
				WillOverwrite bool   `json:"will_overwrite"`
			} `json:"output"`
		} `json:"data"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("expected valid JSON, got %v:\n%s", err, output)
	}
	if !envelope.Success || envelope.Error != nil {
		t.Fatalf("expected success envelope, got:\n%s", output)
	}
	if envelope.Data.PaymentDetails.Payee != "ACME BV" {
		t.Fatalf("expected normalized payee, got %q", envelope.Data.PaymentDetails.Payee)
	}
	if envelope.Data.PaymentDetails.IBAN != "BE68539007547034" {
		t.Fatalf("expected normalized IBAN, got %q", envelope.Data.PaymentDetails.IBAN)
	}
	if envelope.Data.PaymentDetails.Amount != "42.50" {
		t.Fatalf("expected normalized amount, got %q", envelope.Data.PaymentDetails.Amount)
	}
	if envelope.Data.PaymentDetails.Reference.Value != "INV-1" || envelope.Data.PaymentDetails.Reference.Kind != "unstructured" {
		t.Fatalf("expected unstructured reference, got %+v", envelope.Data.PaymentDetails.Reference)
	}
	if envelope.Data.EPC.ServiceTag != "BCD" || envelope.Data.EPC.Version != "002" || envelope.Data.EPC.CharacterSet != "1" {
		t.Fatalf("expected EPC metadata, got %+v", envelope.Data.EPC)
	}
	if envelope.Data.EPC.Identification != "SCT" || envelope.Data.EPC.Currency != "EUR" {
		t.Fatalf("expected EPC SCT EUR metadata, got %+v", envelope.Data.EPC)
	}
	if !strings.Contains(envelope.Data.EPC.Payload, "\nACME BV\nBE68539007547034\nEUR42.50\n") {
		t.Fatalf("expected raw EPC payload with normalized details, got %q", envelope.Data.EPC.Payload)
	}
	if envelope.Data.Output.Path != out || envelope.Data.Output.Format != "svg" {
		t.Fatalf("expected output preflight path and format, got %+v", envelope.Data.Output)
	}
	if envelope.Data.Output.Force || envelope.Data.Output.Exists || envelope.Data.Output.IsSymlink || envelope.Data.Output.WillOverwrite {
		t.Fatalf("expected non-overwrite output preflight, got %+v", envelope.Data.Output)
	}
}

func TestGenerateDryRunJSONValidationFailurePrintsErrorEnvelope(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command(binary, "generate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547035",
		"--amount", "42.50",
		"--reference", "INV-1",
		"--out", out,
		"--dry-run",
		"--json",
	)
	cmd.Stdin = strings.NewReader("yes\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected dry-run validation failure")
	}
	assertGenerateJSONError(t, stdout.Bytes(), "generation_error", "iban", "invalid checksum")
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
	if strings.Contains(stdout.String(), "Write QR artifact") {
		t.Fatalf("expected no prompt, got:\n%s", stdout.String())
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no dry-run output file, got stat err %v", statErr)
	}
}

func TestGenerateDryRunJSONExistingOutputPrintsErrorEnvelopeAndDoesNotWrite(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "invoice.svg")
	if err := os.WriteFile(out, []byte("seed"), 0o644); err != nil {
		t.Fatalf("seed output: %v", err)
	}
	cmd := exec.Command(binary, "generate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547034",
		"--amount", "42.50",
		"--reference", "INV-1",
		"--out", out,
		"--dry-run",
		"--json",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected dry-run existing output failure")
	}
	assertGenerateJSONError(t, stdout.Bytes(), "generation_error", "out", "already exists")
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
	got, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("read output: %v", readErr)
	}
	if string(got) != "seed" {
		t.Fatalf("expected dry-run to leave output unchanged, got %q", got)
	}
}

func TestGenerateDryRunJSONUnknownExtensionPrintsErrorEnvelopeAndDoesNotWrite(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "invoice.qr")
	cmd := exec.Command(binary, "generate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547034",
		"--amount", "42.50",
		"--reference", "INV-1",
		"--out", out,
		"--dry-run",
		"--json",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected dry-run unknown extension failure")
	}
	assertGenerateJSONError(t, stdout.Bytes(), "generation_error", "format", "required")
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no dry-run output file, got stat err %v", statErr)
	}
}

func TestGenerateDryRunJSONMissingParentPrintsErrorEnvelopeAndDoesNotWrite(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "missing", "invoice.svg")
	cmd := exec.Command(binary, "generate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547034",
		"--amount", "42.50",
		"--reference", "INV-1",
		"--out", out,
		"--dry-run",
		"--json",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected dry-run missing parent failure")
	}
	assertGenerateJSONError(t, stdout.Bytes(), "generation_error", "out", "parent directory")
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no dry-run output file, got stat err %v", statErr)
	}
}

func TestGenerateDryRunJSONDirectoryOutputPrintsErrorEnvelope(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "invoice.svg")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatalf("seed output directory: %v", err)
	}
	cmd := exec.Command(binary, "generate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547034",
		"--amount", "42.50",
		"--reference", "INV-1",
		"--out", out,
		"--force",
		"--dry-run",
		"--json",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected dry-run directory output failure")
	}
	assertGenerateJSONError(t, stdout.Bytes(), "generation_error", "out", "directory")
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
	info, statErr := os.Stat(out)
	if statErr != nil {
		t.Fatalf("stat output directory: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatalf("expected dry-run to leave output directory unchanged")
	}
}

func TestGenerateYesJSONWritesArtifactAndPrintsResultEnvelope(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command("go", "run", ".", "generate",
		"--payee", " ACME BV ",
		"--iban", "be68 5390 0754 7034",
		"--amount", "42.5",
		"--reference", "INV-1",
		"--out", out,
		"--yes",
		"--json",
	)

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected generate JSON to succeed, output:\n%s", output)
	}
	if strings.Contains(string(output), "Write QR artifact") || strings.Contains(string(output), "Payment Details") {
		t.Fatalf("expected JSON-only stdout, got:\n%s", output)
	}
	assertSVGOutput(t, out)

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			PaymentDetails paymentDetailsJSON `json:"payment_details"`
			EPC            struct {
				ServiceTag     string `json:"service_tag"`
				Version        string `json:"version"`
				CharacterSet   string `json:"character_set"`
				Identification string `json:"identification"`
				Currency       string `json:"currency"`
			} `json:"epc"`
			Output struct {
				Path      string `json:"path"`
				Format    string `json:"format"`
				ByteCount int    `json:"byte_count"`
			} `json:"output"`
		} `json:"data"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("expected valid JSON, got %v:\n%s", err, output)
	}
	if !envelope.Success || envelope.Error != nil {
		t.Fatalf("expected success envelope, got:\n%s", output)
	}
	if envelope.Data.PaymentDetails.Payee != "ACME BV" || envelope.Data.PaymentDetails.IBAN != "BE68539007547034" {
		t.Fatalf("expected normalized payment details, got %+v", envelope.Data.PaymentDetails)
	}
	if envelope.Data.PaymentDetails.Reference.Kind != "unstructured" {
		t.Fatalf("expected reference kind, got %+v", envelope.Data.PaymentDetails.Reference)
	}
	if envelope.Data.EPC.ServiceTag != "BCD" || envelope.Data.EPC.Version != "002" || envelope.Data.EPC.Identification != "SCT" {
		t.Fatalf("expected EPC metadata, got %+v", envelope.Data.EPC)
	}
	if envelope.Data.Output.Path != out || envelope.Data.Output.Format != "svg" {
		t.Fatalf("expected artifact output metadata, got %+v", envelope.Data.Output)
	}
	if envelope.Data.Output.ByteCount <= 0 {
		t.Fatalf("expected byte count, got %+v", envelope.Data.Output)
	}
}

func TestGenerateJSONPromptsOnStderrAndPrintsResultEnvelope(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command("go", "run", ".", "generate",
		"--payee", " ACME BV ",
		"--iban", "be68 5390 0754 7034",
		"--amount", "42.5",
		"--reference", "INV-1",
		"--out", out,
		"--json",
	)
	cmd.Stdin = strings.NewReader("yes\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected generate JSON to succeed, stdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "Write QR artifact") || strings.Contains(stdout.String(), "Payment Details") {
		t.Fatalf("expected JSON-only stdout, got:\n%s", stdout.String())
	}
	for _, want := range []string{"Payment Details", "Payee: ACME BV", "Write QR artifact?"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr.String())
		}
	}
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Output struct {
				Path      string `json:"path"`
				Format    string `json:"format"`
				ByteCount int    `json:"byte_count"`
			} `json:"output"`
		} `json:"data"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid JSON stdout, got %v:\n%s", err, stdout.String())
	}
	if !envelope.Success || envelope.Error != nil {
		t.Fatalf("expected success envelope, got:\n%s", stdout.String())
	}
	if envelope.Data.Output.Path != out || envelope.Data.Output.Format != "svg" || envelope.Data.Output.ByteCount <= 0 {
		t.Fatalf("expected artifact output metadata, got %+v", envelope.Data.Output)
	}
	assertSVGOutput(t, out)
}

func TestGenerateJSONRefusalPrintsErrorEnvelopeAndDoesNotWrite(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command(binary, "generate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547034",
		"--amount", "42.50",
		"--reference", "INV-1",
		"--out", out,
		"--json",
	)
	cmd.Stdin = strings.NewReader("no\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected confirmation refusal")
	}
	assertGenerateJSONError(t, stdout.Bytes(), "generation_error", "confirmation", "refused")
	if !strings.Contains(stderr.String(), "Write QR artifact?") {
		t.Fatalf("expected confirmation prompt on stderr, got:\n%s", stderr.String())
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestGenerateJSONValidationFailureDoesNotPromptOrWrite(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command(binary, "generate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547035",
		"--amount", "42.50",
		"--reference", "INV-1",
		"--out", out,
		"--json",
	)
	cmd.Stdin = strings.NewReader("yes\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected validation failure")
	}
	assertGenerateJSONError(t, stdout.Bytes(), "generation_error", "iban", "invalid checksum")
	if stderr.Len() != 0 {
		t.Fatalf("expected no confirmation stderr, got:\n%s", stderr.String())
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestGenerateJSONPreflightFailureDoesNotPromptOrWrite(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "invoice.svg")
	if err := os.WriteFile(out, []byte("seed"), 0o644); err != nil {
		t.Fatalf("seed output: %v", err)
	}
	cmd := exec.Command(binary, "generate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547034",
		"--amount", "42.50",
		"--reference", "INV-1",
		"--out", out,
		"--json",
	)
	cmd.Stdin = strings.NewReader("yes\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected output preflight failure")
	}
	assertGenerateJSONError(t, stdout.Bytes(), "generation_error", "out", "already exists")
	if stderr.Len() != 0 {
		t.Fatalf("expected no confirmation stderr, got:\n%s", stderr.String())
	}
	got, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("read output: %v", readErr)
	}
	if string(got) != "seed" {
		t.Fatalf("expected preflight failure to leave output unchanged, got %q", got)
	}
}

func TestGeneratePromptsAndWritesAfterConfirmation(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command("go", "run", ".", "generate",
		"--payee", " ACME BV ",
		"--iban", "be68 5390 0754 7034",
		"--amount", "42.5",
		"--reference", "INV-1",
		"--out", out,
	)
	cmd.Stdin = strings.NewReader("yes\n")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected generate to succeed, output:\n%s", output)
	}
	for _, want := range []string{
		"Payment Details",
		"Payee: ACME BV",
		"IBAN: BE68539007547034",
		"Amount: EUR42.50",
		"Reference: INV-1",
		"Write QR artifact?",
	} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}
	assertSVGOutput(t, out)
}

func TestGenerateRefusalDoesNotWrite(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command("go", "run", ".", "generate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547034",
		"--amount", "42.50",
		"--reference", "INV-1",
		"--out", out,
	)
	cmd.Stdin = strings.NewReader("no\n")

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected refusal to fail, output:\n%s", output)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestGenerateValidationFailureDoesNotPromptOrWrite(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command("go", "run", ".", "generate",
		"--payee", "ACME BV",
		"--iban", "BE68539007547035",
		"--amount", "42.50",
		"--reference", "INV-1",
		"--out", out,
	)
	cmd.Stdin = strings.NewReader("yes\n")

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected validation failure, output:\n%s", output)
	}
	if strings.Contains(string(output), "Write QR artifact") {
		t.Fatalf("expected validation before prompt, got:\n%s", output)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestFromTextFilePromptsAndWritesAfterConfirmation(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "invoice.txt")
	out := filepath.Join(dir, "invoice.svg")
	if err := os.WriteFile(input, []byte(clearInvoiceText()), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	cmd := exec.Command("go", "run", ".", "from-text", input, "--out", out)
	cmd.Stdin = strings.NewReader("yes\n")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected from-text to succeed, output:\n%s", output)
	}
	for _, want := range []string{
		"Payment Details",
		"Payee: ACME BV",
		"IBAN: BE68539007547034",
		"Amount: EUR42.50",
		"Reference: INV-2026-001",
		"Write QR artifact?",
	} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}
	assertSVGOutput(t, out)
}

func TestFromTextDryRunJSONPrintsSuggestionsWithEvidenceAndPlan(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "invoice.txt")
	out := filepath.Join(dir, "invoice.svg")
	if err := os.WriteFile(input, []byte(clearInvoiceText()), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	cmd := exec.Command("go", "run", ".", "from-text", input, "--out", out, "--dry-run", "--json")
	cmd.Stdin = strings.NewReader("yes\n")

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected from-text dry-run JSON to succeed, output:\n%s", output)
	}
	if strings.Contains(string(output), "Write QR artifact") || strings.Contains(string(output), "Payment Details") {
		t.Fatalf("expected JSON-only dry-run output, got:\n%s", output)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no dry-run output file, got stat err %v", statErr)
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Suggestions struct {
				Payee struct {
					Value    string `json:"value"`
					Source   string `json:"source"`
					Evidence string `json:"evidence"`
				} `json:"payee"`
				IBAN struct {
					Value    string `json:"value"`
					Source   string `json:"source"`
					Evidence string `json:"evidence"`
				} `json:"iban"`
				Amount struct {
					Value    string `json:"value"`
					Source   string `json:"source"`
					Evidence string `json:"evidence"`
				} `json:"amount"`
				Reference struct {
					Value    string `json:"value"`
					Source   string `json:"source"`
					Evidence string `json:"evidence"`
				} `json:"reference"`
			} `json:"suggestions"`
			Plan struct {
				PaymentDetails paymentDetailsJSON `json:"payment_details"`
				EPC            struct {
					Payload string `json:"payload"`
				} `json:"epc"`
				Output struct {
					Path   string `json:"path"`
					Format string `json:"format"`
				} `json:"output"`
			} `json:"plan"`
		} `json:"data"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("expected valid JSON, got %v:\n%s", err, output)
	}
	if !envelope.Success || envelope.Error != nil {
		t.Fatalf("expected success envelope, got:\n%s", output)
	}
	if envelope.Data.Suggestions.Payee.Value != "ACME BV" || envelope.Data.Suggestions.Payee.Source != "text" {
		t.Fatalf("unexpected payee suggestion: %+v", envelope.Data.Suggestions.Payee)
	}
	if !strings.Contains(envelope.Data.Suggestions.Payee.Evidence, "Payee: ACME BV") {
		t.Fatalf("expected payee evidence, got %+v", envelope.Data.Suggestions.Payee)
	}
	if envelope.Data.Suggestions.IBAN.Value != "BE68 5390 0754 7034" || envelope.Data.Suggestions.IBAN.Source != "text" {
		t.Fatalf("unexpected iban suggestion: %+v", envelope.Data.Suggestions.IBAN)
	}
	if !strings.Contains(envelope.Data.Suggestions.IBAN.Evidence, "IBAN: BE68 5390 0754 7034") {
		t.Fatalf("expected iban evidence, got %+v", envelope.Data.Suggestions.IBAN)
	}
	if envelope.Data.Suggestions.Amount.Value != "42.50" || envelope.Data.Suggestions.Amount.Source != "text" {
		t.Fatalf("unexpected amount suggestion: %+v", envelope.Data.Suggestions.Amount)
	}
	if !strings.Contains(envelope.Data.Suggestions.Amount.Evidence, "Amount: EUR 42.50") {
		t.Fatalf("expected amount evidence, got %+v", envelope.Data.Suggestions.Amount)
	}
	if envelope.Data.Suggestions.Reference.Value != "INV-2026-001" || envelope.Data.Suggestions.Reference.Source != "text" {
		t.Fatalf("unexpected reference suggestion: %+v", envelope.Data.Suggestions.Reference)
	}
	if envelope.Data.Plan.PaymentDetails.IBAN != "BE68539007547034" || envelope.Data.Plan.PaymentDetails.Amount != "42.50" {
		t.Fatalf("expected validated payment details in plan, got %+v", envelope.Data.Plan.PaymentDetails)
	}
	if !strings.Contains(envelope.Data.Plan.EPC.Payload, "\nACME BV\nBE68539007547034\nEUR42.50\n") {
		t.Fatalf("expected EPC payload in plan, got %q", envelope.Data.Plan.EPC.Payload)
	}
	if envelope.Data.Plan.Output.Path != out || envelope.Data.Plan.Output.Format != "svg" {
		t.Fatalf("expected output plan, got %+v", envelope.Data.Plan.Output)
	}
}

func TestFromTextDryRunJSONMarksExplicitOverrides(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "invoice.txt")
	out := filepath.Join(dir, "invoice.svg")
	if err := os.WriteFile(input, []byte(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	cmd := exec.Command("go", "run", ".", "from-text", input,
		"--amount", "10",
		"--reference", "MANUAL-REF",
		"--out", out,
		"--dry-run",
		"--json",
	)

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected from-text dry-run JSON to succeed, output:\n%s", output)
	}
	var envelope struct {
		Data struct {
			Suggestions struct {
				Amount struct {
					Value    string `json:"value"`
					Source   string `json:"source"`
					Evidence string `json:"evidence"`
				} `json:"amount"`
				Reference struct {
					Value    string `json:"value"`
					Source   string `json:"source"`
					Evidence string `json:"evidence"`
				} `json:"reference"`
				IBAN struct {
					Source   string `json:"source"`
					Evidence string `json:"evidence"`
				} `json:"iban"`
			} `json:"suggestions"`
			Plan struct {
				PaymentDetails paymentDetailsJSON `json:"payment_details"`
			} `json:"plan"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("expected valid JSON, got %v:\n%s", err, output)
	}
	if envelope.Data.Suggestions.Amount.Value != "10" || envelope.Data.Suggestions.Amount.Source != "override" || envelope.Data.Suggestions.Amount.Evidence != "" {
		t.Fatalf("unexpected amount override: %+v", envelope.Data.Suggestions.Amount)
	}
	if envelope.Data.Suggestions.Reference.Value != "MANUAL-REF" || envelope.Data.Suggestions.Reference.Source != "override" || envelope.Data.Suggestions.Reference.Evidence != "" {
		t.Fatalf("unexpected reference override: %+v", envelope.Data.Suggestions.Reference)
	}
	if envelope.Data.Suggestions.IBAN.Source != "text" || envelope.Data.Suggestions.IBAN.Evidence == "" {
		t.Fatalf("expected text-derived iban evidence, got %+v", envelope.Data.Suggestions.IBAN)
	}
	if envelope.Data.Plan.PaymentDetails.Amount != "10.00" || envelope.Data.Plan.PaymentDetails.Reference.Value != "MANUAL-REF" {
		t.Fatalf("expected override values in validated plan, got %+v", envelope.Data.Plan.PaymentDetails)
	}
}

func TestFromTextDryRunJSONAmbiguousSuggestionPrintsErrorEnvelope(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command(binary, "from-text", "--out", out, "--dry-run", "--json")
	cmd.Stdin = strings.NewReader(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Total: EUR 12.00
Reference: INV-1
`)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected ambiguity failure")
	}
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Suggestions struct {
				Payee struct {
					Value string `json:"value"`
				} `json:"payee"`
				Amount struct {
					Value string `json:"value"`
				} `json:"amount"`
			} `json:"suggestions"`
			MissingFields   []string `json:"missing_fields"`
			AmbiguousFields []string `json:"ambiguous_fields"`
			AgentContext    struct {
				Candidates struct {
					Amount []struct {
						Value      string `json:"value"`
						Normalized string `json:"normalized"`
					} `json:"amount"`
				} `json:"candidates"`
			} `json:"agent_context"`
			Plan any `json:"plan"`
		} `json:"data"`
		Error cliErrorJSON `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid JSON stdout, got %v:\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if envelope.Success || envelope.Error.Code != "incomplete_suggestion" || envelope.Error.Field != "amount" || envelope.Error.Message != "ambiguous" {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
	if envelope.Data.Suggestions.Payee.Value != "ACME BV" || envelope.Data.Suggestions.Amount.Value != "" {
		t.Fatalf("unexpected partial suggestions: %+v", envelope.Data.Suggestions)
	}
	if len(envelope.Data.MissingFields) != 0 || strings.Join(envelope.Data.AmbiguousFields, ",") != "amount" {
		t.Fatalf("unexpected incomplete fields: missing=%v ambiguous=%v", envelope.Data.MissingFields, envelope.Data.AmbiguousFields)
	}
	if envelope.Data.Plan != nil {
		t.Fatalf("expected no plan for incomplete suggestion, got %#v", envelope.Data.Plan)
	}
	if len(envelope.Data.AgentContext.Candidates.Amount) != 2 ||
		envelope.Data.AgentContext.Candidates.Amount[0].Normalized != "42.50" ||
		envelope.Data.AgentContext.Candidates.Amount[1].Normalized != "12.00" {
		t.Fatalf("unexpected amount candidates: %+v", envelope.Data.AgentContext.Candidates.Amount)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
	if strings.Contains(stdout.String(), "Payment Details") || strings.Contains(stdout.String(), "Write QR artifact") {
		t.Fatalf("expected JSON-only error output, got:\n%s", stdout.String())
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestFromTextDryRunJSONAmbiguousIBANPrintsErrorEnvelope(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command(binary, "from-text", "--out", out, "--dry-run", "--json")
	cmd.Stdin = strings.NewReader(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Alternative IBAN: NL91 ABNA 0417 1643 00
Amount: EUR 42.50
Reference: INV-1
`)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected iban ambiguity failure")
	}
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			AmbiguousFields []string `json:"ambiguous_fields"`
			AgentContext    struct {
				Candidates struct {
					IBAN []struct {
						Normalized string `json:"normalized"`
					} `json:"iban"`
				} `json:"candidates"`
			} `json:"agent_context"`
			Plan any `json:"plan"`
		} `json:"data"`
		Error cliErrorJSON `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid JSON stdout, got %v:\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if envelope.Success || envelope.Error.Code != "incomplete_suggestion" || envelope.Error.Field != "iban" || envelope.Error.Message != "ambiguous" {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
	if strings.Join(envelope.Data.AmbiguousFields, ",") != "iban" {
		t.Fatalf("unexpected ambiguous fields: %v", envelope.Data.AmbiguousFields)
	}
	if len(envelope.Data.AgentContext.Candidates.IBAN) != 2 ||
		envelope.Data.AgentContext.Candidates.IBAN[0].Normalized != "BE68539007547034" ||
		envelope.Data.AgentContext.Candidates.IBAN[1].Normalized != "NL91ABNA0417164300" {
		t.Fatalf("unexpected iban candidates: %+v", envelope.Data.AgentContext.Candidates.IBAN)
	}
	if envelope.Data.Plan != nil {
		t.Fatalf("expected no plan for incomplete suggestion, got %#v", envelope.Data.Plan)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestFromTextDryRunJSONMissingSuggestionPrintsErrorEnvelope(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command(binary, "from-text", "--out", out, "--dry-run", "--json")
	cmd.Stdin = strings.NewReader(`
Payee: ACME BV
Amount: EUR 42.50
Reference: INV-1
`)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected missing-field failure")
	}
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Suggestions struct {
				Payee struct {
					Value string `json:"value"`
				} `json:"payee"`
				IBAN struct {
					Value string `json:"value"`
				} `json:"iban"`
				Amount struct {
					Value string `json:"value"`
				} `json:"amount"`
				Reference struct {
					Value string `json:"value"`
				} `json:"reference"`
			} `json:"suggestions"`
			MissingFields   []string `json:"missing_fields"`
			AmbiguousFields []string `json:"ambiguous_fields"`
			AgentContext    struct {
				SourceTextHash string `json:"source_text_hash"`
				Candidates     struct {
					Amount []struct {
						Value      string `json:"value"`
						Normalized string `json:"normalized"`
						Line       int    `json:"line"`
					} `json:"amount"`
				} `json:"candidates"`
			} `json:"agent_context"`
			Plan any `json:"plan"`
		} `json:"data"`
		Error cliErrorJSON `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid JSON stdout, got %v:\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if envelope.Success || envelope.Error.Code != "incomplete_suggestion" || envelope.Error.Field != "iban" || envelope.Error.Message != "required" {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
	if envelope.Data.Suggestions.Payee.Value != "ACME BV" || envelope.Data.Suggestions.Amount.Value != "42.50" ||
		envelope.Data.Suggestions.Reference.Value != "INV-1" || envelope.Data.Suggestions.IBAN.Value != "" {
		t.Fatalf("unexpected partial suggestions: %+v", envelope.Data.Suggestions)
	}
	if strings.Join(envelope.Data.MissingFields, ",") != "iban" || len(envelope.Data.AmbiguousFields) != 0 {
		t.Fatalf("unexpected incomplete fields: missing=%v ambiguous=%v", envelope.Data.MissingFields, envelope.Data.AmbiguousFields)
	}
	if envelope.Data.Plan != nil {
		t.Fatalf("expected no plan for incomplete suggestion, got %#v", envelope.Data.Plan)
	}
	if envelope.Data.AgentContext.SourceTextHash == "" || len(envelope.Data.AgentContext.Candidates.Amount) != 1 ||
		envelope.Data.AgentContext.Candidates.Amount[0].Value != "42.50" ||
		envelope.Data.AgentContext.Candidates.Amount[0].Normalized != "42.50" ||
		envelope.Data.AgentContext.Candidates.Amount[0].Line == 0 {
		t.Fatalf("unexpected agent context: %+v", envelope.Data.AgentContext)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestFromTextDryRunJSONInvalidCompleteSuggestionOmitsEPCPayload(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command(binary, "from-text",
		"--amount", "0",
		"--out", out,
		"--dry-run",
		"--json",
	)
	cmd.Stdin = strings.NewReader(clearInvoiceText())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected validation failure")
	}
	if strings.Contains(stdout.String(), "payload") || strings.Contains(stdout.String(), "BCD") {
		t.Fatalf("expected no EPC payload data, got:\n%s", stdout.String())
	}
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Suggestions struct {
				Amount struct {
					Value  string `json:"value"`
					Source string `json:"source"`
				} `json:"amount"`
			} `json:"suggestions"`
			Plan any `json:"plan"`
		} `json:"data"`
		Error cliErrorJSON `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid JSON, got %v:\n%s", err, stdout.String())
	}
	if envelope.Success || envelope.Error.Code != "generation_error" || envelope.Error.Field != "amount" || !strings.Contains(envelope.Error.Message, "greater than zero") {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
	if envelope.Data.Suggestions.Amount.Value != "0" || envelope.Data.Suggestions.Amount.Source != "override" {
		t.Fatalf("expected invalid override suggestion to remain visible, got %+v", envelope.Data.Suggestions.Amount)
	}
	if envelope.Data.Plan != nil {
		t.Fatalf("expected no plan for invalid complete suggestion, got %#v", envelope.Data.Plan)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestFromTextDryRunWithoutJSONFailsBeforeReadingInput(t *testing.T) {
	err := FromTextCmd{
		File:   filepath.Join(t.TempDir(), "missing.txt"),
		DryRun: true,
	}.Run()

	if err == nil {
		t.Fatalf("expected dry-run/json flag error")
	}
	if !strings.Contains(err.Error(), "dry-run") || !strings.Contains(err.Error(), "--json") {
		t.Fatalf("expected dry-run JSON error, got %v", err)
	}
}

func TestFromTextJSONWithoutDryRunFailsBeforeReadingInput(t *testing.T) {
	err := FromTextCmd{
		File: filepath.Join(t.TempDir(), "missing.txt"),
		JSON: true,
	}.Run()

	if err == nil {
		t.Fatalf("expected json/dry-run flag error")
	}
	if !strings.Contains(err.Error(), "json") || !strings.Contains(err.Error(), "--dry-run") {
		t.Fatalf("expected JSON dry-run error, got %v", err)
	}
}

func TestFromTextDryRunJSONReadFailurePrintsErrorEnvelope(t *testing.T) {
	binary := buildInvoiceqrCLI(t)
	cmd := exec.Command(binary, "from-text", filepath.Join(t.TempDir(), "missing.txt"), "--out", "invoice.svg", "--dry-run", "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected read failure")
	}
	assertGenerateJSONError(t, stdout.Bytes(), "input_error", "", "no such file")
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
}

func TestFromTextStdinUsesOverridesAndTerminalConfirmation(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "invoice.svg")
	input, err := os.CreateTemp(dir, "invoice-*.txt")
	if err != nil {
		t.Fatalf("create input: %v", err)
	}
	if _, err := input.WriteString(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Total: EUR 12.00
Reference: INV-2026-001
`); err != nil {
		t.Fatalf("write input: %v", err)
	}
	if _, err := input.Seek(0, 0); err != nil {
		t.Fatalf("seek input: %v", err)
	}
	confirmationPath := filepath.Join(dir, "confirmation.txt")
	if err := os.WriteFile(confirmationPath, []byte("yes\n"), 0o644); err != nil {
		t.Fatalf("write confirmation: %v", err)
	}

	oldStdin := os.Stdin
	oldTerminalInputPath := terminalInputPath
	os.Stdin = input
	terminalInputPath = confirmationPath
	defer func() {
		os.Stdin = oldStdin
		terminalInputPath = oldTerminalInputPath
		input.Close()
	}()

	cmd := FromTextCmd{
		PaymentDetailsFlags: PaymentDetailsFlags{
			Amount:    "10",
			Reference: "MANUAL-REF",
		},
		QROutputFlags: QROutputFlags{Out: out},
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected from-text stdin to succeed, got %v", err)
	}
	assertSVGOutput(t, out)
}

func TestFromTextMissingFieldDoesNotPromptOrWrite(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command("go", "run", ".", "from-text", "--out", out)
	cmd.Stdin = strings.NewReader("Payee: ACME BV\nAmount: EUR 42.50\nReference: INV-1\nyes\n")

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing field failure, output:\n%s", output)
	}
	if !strings.Contains(strings.ToLower(string(output)), "iban") {
		t.Fatalf("expected iban error, got:\n%s", output)
	}
	if strings.Contains(string(output), "Write QR artifact") {
		t.Fatalf("expected no prompt before complete suggestion, got:\n%s", output)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestFromTextAmbiguousAmountDoesNotWrite(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	cmd := exec.Command("go", "run", ".", "from-text", "--out", out)
	cmd.Stdin = strings.NewReader(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Total: EUR 12.00
Reference: INV-1
yes
`)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected ambiguity failure, output:\n%s", output)
	}
	if !strings.Contains(strings.ToLower(string(output)), "amount") || !strings.Contains(strings.ToLower(string(output)), "ambiguous") {
		t.Fatalf("expected amount ambiguity error, got:\n%s", output)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestFromTextRejectsYesFlag(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "from-text", "--yes")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected --yes to fail for from-text, output:\n%s", output)
	}
	if !strings.Contains(strings.ToLower(string(output)), "unknown flag") || !strings.Contains(string(output), "--yes") {
		t.Fatalf("expected unknown --yes flag error, output:\n%s", output)
	}
}

func TestFromPDFInvokesPdftotextAndWritesAfterConfirmation(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	setStdin(t, "yes\n")
	var gotName string
	var gotArgs []string
	withPDFTextCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = args
		return []byte(clearInvoiceText()), nil
	})

	cmd := FromPDFCmd{
		PDF:           "invoice.pdf",
		QROutputFlags: QROutputFlags{Out: out},
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected from-pdf to succeed, got %v", err)
	}
	if gotName != "pdftotext" {
		t.Fatalf("runner name = %q, want pdftotext", gotName)
	}
	if strings.Join(gotArgs, "\x00") != strings.Join([]string{"invoice.pdf", "-"}, "\x00") {
		t.Fatalf("runner args = %#v, want %#v", gotArgs, []string{"invoice.pdf", "-"})
	}
	assertSVGOutput(t, out)
}

func TestFromPDFDryRunJSONUsesExtractedTextEvidence(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	withPDFTextCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		if name != "pdftotext" || strings.Join(args, "\x00") != strings.Join([]string{"invoice.pdf", "-"}, "\x00") {
			t.Fatalf("unexpected pdftotext call %s %#v", name, args)
		}
		return []byte(clearInvoiceText()), nil
	})

	output := captureStdout(t, func() error {
		return FromPDFCmd{
			PDF:           "invoice.pdf",
			QROutputFlags: QROutputFlags{Out: out},
			DryRun:        true,
			JSON:          true,
		}.Run()
	})

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Suggestions struct {
				IBAN struct {
					Value    string `json:"value"`
					Source   string `json:"source"`
					Evidence string `json:"evidence"`
				} `json:"iban"`
			} `json:"suggestions"`
			Plan struct {
				EPC struct {
					Payload string `json:"payload"`
				} `json:"epc"`
			} `json:"plan"`
		} `json:"data"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("expected valid JSON, got %v:\n%s", err, output)
	}
	if !envelope.Success || envelope.Error != nil {
		t.Fatalf("expected success envelope, got:\n%s", output)
	}
	if envelope.Data.Suggestions.IBAN.Value != "BE68 5390 0754 7034" || envelope.Data.Suggestions.IBAN.Source != "text" {
		t.Fatalf("unexpected iban suggestion: %+v", envelope.Data.Suggestions.IBAN)
	}
	if !strings.Contains(envelope.Data.Suggestions.IBAN.Evidence, "IBAN: BE68 5390 0754 7034") {
		t.Fatalf("expected extracted text evidence, got %+v", envelope.Data.Suggestions.IBAN)
	}
	if strings.Contains(strings.ToLower(envelope.Data.Suggestions.IBAN.Evidence), "page") || strings.Contains(envelope.Data.Suggestions.IBAN.Evidence, ",") {
		t.Fatalf("expected text snippet without coordinates, got %+v", envelope.Data.Suggestions.IBAN)
	}
	if !strings.Contains(envelope.Data.Plan.EPC.Payload, "BE68539007547034") {
		t.Fatalf("expected EPC payload, got %q", envelope.Data.Plan.EPC.Payload)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no dry-run output file, got stat err %v", statErr)
	}
}

func TestFromPDFDryRunWithoutJSONFailsBeforeExtraction(t *testing.T) {
	called := false
	withPDFTextCommandRunner(t, func(string, ...string) ([]byte, error) {
		called = true
		return []byte(clearInvoiceText()), nil
	})

	err := FromPDFCmd{
		PDF:    "invoice.pdf",
		DryRun: true,
	}.Run()

	if err == nil {
		t.Fatalf("expected dry-run/json flag error")
	}
	if called {
		t.Fatalf("expected no PDF extraction before flag validation")
	}
	if !strings.Contains(err.Error(), "dry-run") || !strings.Contains(err.Error(), "--json") {
		t.Fatalf("expected dry-run JSON error, got %v", err)
	}
}

func TestFromPDFJSONWithoutDryRunFailsBeforeExtraction(t *testing.T) {
	called := false
	withPDFTextCommandRunner(t, func(string, ...string) ([]byte, error) {
		called = true
		return []byte(clearInvoiceText()), nil
	})

	err := FromPDFCmd{
		PDF:  "invoice.pdf",
		JSON: true,
	}.Run()

	if err == nil {
		t.Fatalf("expected json/dry-run flag error")
	}
	if called {
		t.Fatalf("expected no PDF extraction before flag validation")
	}
	if !strings.Contains(err.Error(), "json") || !strings.Contains(err.Error(), "--dry-run") {
		t.Fatalf("expected JSON dry-run error, got %v", err)
	}
}

func TestFromPDFDryRunJSONExtractionFailurePrintsErrorEnvelope(t *testing.T) {
	withPDFTextCommandRunner(t, func(string, ...string) ([]byte, error) {
		return nil, exec.ErrNotFound
	})

	output, err := captureStdoutAndError(t, func() error {
		return FromPDFCmd{
			PDF:           "invoice.pdf",
			QROutputFlags: QROutputFlags{Out: "invoice.svg"},
			DryRun:        true,
			JSON:          true,
		}.Run()
	})

	if err == nil {
		t.Fatalf("expected extraction failure")
	}
	assertGenerateJSONError(t, output, "extraction_error", "", "pdftotext")
}

func TestFromPDFExtractsGeneratedInvoicePDFAndWritesAfterConfirmation(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed")
	}

	dir := t.TempDir()
	pdf := filepath.Join(dir, "invoice.pdf")
	out := filepath.Join(dir, "invoice.svg")
	writeInvoicePDF(t, pdf, []string{
		"Payee: ACME BV",
		"IBAN: BE68 5390 0754 7034",
		"Amount: EUR 42.50",
		"Reference: INV-2026-001",
	})
	setStdin(t, "yes\n")

	cmd := FromPDFCmd{
		PDF:           pdf,
		QROutputFlags: QROutputFlags{Out: out},
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected generated PDF extraction to succeed, got %v", err)
	}
	assertSVGOutput(t, out)
}

func TestFromPDFMissingPdftotextReportsInstallHelp(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	withPDFTextCommandRunner(t, func(string, ...string) ([]byte, error) {
		return nil, exec.ErrNotFound
	})

	cmd := FromPDFCmd{
		PDF:           "invoice.pdf",
		QROutputFlags: QROutputFlags{Out: out},
	}
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected missing pdftotext error")
	}
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "pdftotext") || !strings.Contains(message, "install") {
		t.Fatalf("expected pdftotext install help, got %v", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestFromPDFAmbiguousSuggestionDoesNotWrite(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	setStdin(t, "yes\n")
	withPDFTextCommandRunner(t, func(string, ...string) ([]byte, error) {
		return []byte(`
Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Total: EUR 12.00
Reference: INV-1
`), nil
	})

	cmd := FromPDFCmd{
		PDF:           "invoice.pdf",
		QROutputFlags: QROutputFlags{Out: out},
	}
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected ambiguity error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "amount") || !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Fatalf("expected amount ambiguity error, got %v", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestFromPDFRefusalDoesNotWrite(t *testing.T) {
	out := filepath.Join(t.TempDir(), "invoice.svg")
	setStdin(t, "no\n")
	withPDFTextCommandRunner(t, func(string, ...string) ([]byte, error) {
		return []byte(clearInvoiceText()), nil
	})

	cmd := FromPDFCmd{
		PDF:           "invoice.pdf",
		QROutputFlags: QROutputFlags{Out: out},
	}
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected confirmation refusal")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "confirmation") {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file, got stat err %v", statErr)
	}
}

func TestFromPDFRejectsYesFlag(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "from-pdf", "invoice.pdf", "--yes")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected --yes to fail for from-pdf, output:\n%s", output)
	}
	if !strings.Contains(strings.ToLower(string(output)), "unknown flag") || !strings.Contains(string(output), "--yes") {
		t.Fatalf("expected unknown --yes flag error, output:\n%s", output)
	}
}

func TestConfirmationReturnsReadErrors(t *testing.T) {
	readErr := errors.New("terminal read failed")
	confirmed, err := confirmPaymentDetailsWithInput(validConfirmationDetails(), &failingReader{
		data: []byte("yes"),
		err:  readErr,
	})

	if !errors.Is(err, readErr) {
		t.Fatalf("expected read error, got %v", err)
	}
	if confirmed {
		t.Fatalf("expected no confirmation when read fails")
	}
}

func TestConfirmationAcceptsEOFAfterAnswer(t *testing.T) {
	confirmed, err := confirmPaymentDetailsWithInput(validConfirmationDetails(), strings.NewReader("yes"))

	if err != nil {
		t.Fatalf("expected EOF after answer to be accepted, got %v", err)
	}
	if !confirmed {
		t.Fatalf("expected confirmation")
	}
}

func assertSVGOutput(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "<svg") {
		t.Fatalf("expected SVG output, got %q", data[:min(len(data), 120)])
	}
}

func writeInvoicePDF(t *testing.T, path string, lines []string) {
	t.Helper()

	var content bytes.Buffer
	content.WriteString("BT\n/F1 12 Tf\n16 TL\n72 720 Td\n")
	for i, line := range lines {
		if i > 0 {
			content.WriteString("T*\n")
		}
		fmt.Fprintf(&content, "(%s) Tj\n", escapePDFText(line))
	}
	content.WriteString("ET\n")

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", content.Len(), content.String()),
	}

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 0, len(objects)+1)
	offsets = append(offsets, 0)
	for index, object := range objects {
		offsets = append(offsets, pdf.Len())
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xrefOffset := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n", len(objects)+1)
	pdf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)

	if err := os.WriteFile(path, pdf.Bytes(), 0o644); err != nil {
		t.Fatalf("write PDF: %v", err)
	}
}

func escapePDFText(text string) string {
	text = strings.ReplaceAll(text, `\`, `\\`)
	text = strings.ReplaceAll(text, "(", `\(`)
	return strings.ReplaceAll(text, ")", `\)`)
}

func clearInvoiceText() string {
	return `Payee: ACME BV
IBAN: BE68 5390 0754 7034
Amount: EUR 42.50
Reference: INV-2026-001
`
}

func setStdin(t *testing.T, input string) {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "stdin-*.txt")
	if err != nil {
		t.Fatalf("create stdin: %v", err)
	}
	if _, err := file.WriteString(input); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("seek stdin: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = file
	t.Cleanup(func() {
		os.Stdin = oldStdin
		file.Close()
	})
}

func captureStdout(t *testing.T, run func() error) []byte {
	t.Helper()

	output, err := captureStdoutAndError(t, run)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	return output
}

func captureStdoutAndError(t *testing.T, run func() error) ([]byte, error) {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "stdout-*.txt")
	if err != nil {
		t.Fatalf("create stdout: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = file
	defer func() {
		os.Stdout = oldStdout
		file.Close()
	}()

	runErr := run()
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("seek stdout: %v", err)
	}
	output, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return output, runErr
}

func assertGenerateJSONError(t *testing.T, output []byte, code string, field string, message string) {
	t.Helper()

	var envelope struct {
		Success bool         `json:"success"`
		Data    any          `json:"data"`
		Error   cliErrorJSON `json:"error"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("expected valid JSON error envelope, got %v:\n%s", err, output)
	}
	if envelope.Success {
		t.Fatalf("expected failure envelope, got:\n%s", output)
	}
	if envelope.Data != nil {
		t.Fatalf("expected null data, got:\n%s", output)
	}
	if envelope.Error.Code != code || envelope.Error.Field != field {
		t.Fatalf("expected %s %s error, got %+v", code, field, envelope.Error)
	}
	if !strings.Contains(envelope.Error.Message, message) {
		t.Fatalf("expected error message to contain %q, got %+v", message, envelope.Error)
	}
}

func buildInvoiceqrCLI(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "invoiceqr")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build invoiceqr: %v\n%s", err, output)
	}
	return binary
}

func withPDFTextCommandRunner(t *testing.T, runner commandRunner) {
	t.Helper()

	oldRunner := pdfTextCommandRunner
	pdfTextCommandRunner = runner
	t.Cleanup(func() {
		pdfTextCommandRunner = oldRunner
	})
}

func validConfirmationDetails() invoiceqr.ValidatedPaymentDetails {
	return invoiceqr.ValidatedPaymentDetails{
		Payee:  "ACME BV",
		IBAN:   "BE68539007547034",
		Amount: "42.50",
		Reference: invoiceqr.RemittanceReference{
			Kind:  invoiceqr.UnstructuredReference,
			Value: "INV-1",
		},
	}
}

type failingReader struct {
	data []byte
	err  error
	done bool
}

func (reader *failingReader) Read(p []byte) (int, error) {
	if reader.done {
		return 0, io.EOF
	}
	reader.done = true
	return copy(p, reader.data), reader.err
}

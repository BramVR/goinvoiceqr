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
		name  string
		args  []string
		flags []string
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
	assertGenerateDryRunJSONError(t, stdout.Bytes(), "generation_error", "iban", "invalid checksum")
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
	assertGenerateDryRunJSONError(t, stdout.Bytes(), "generation_error", "out", "already exists")
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
	assertGenerateDryRunJSONError(t, stdout.Bytes(), "generation_error", "format", "required")
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected no dry-run output file, got stat err %v", statErr)
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

func assertGenerateDryRunJSONError(t *testing.T, output []byte, code string, field string, message string) {
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

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func TestCommandsReturnPlaceholderErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "from-text", args: []string{"from-text"}, message: "from-text is not implemented yet"},
		{name: "from-pdf", args: []string{"from-pdf"}, message: "from-pdf is not implemented yet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("go", append([]string{"run", "."}, tt.args...)...)

			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected command to fail, output:\n%s", output)
			}
			if !strings.Contains(string(output), tt.message) {
				t.Fatalf("expected output to contain %q, got:\n%s", tt.message, output)
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

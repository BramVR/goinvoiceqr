package main

import (
	"os/exec"
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
		{name: "generate", args: []string{"generate"}, message: "generate is not implemented yet"},
		{name: "validate", args: []string{"validate"}, message: "validate is not implemented yet"},
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

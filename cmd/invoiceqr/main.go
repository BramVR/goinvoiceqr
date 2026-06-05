package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/bramvr/goinvoiceqr/internal/invoiceqr"
)

type CLI struct {
	Generate GenerateCmd `cmd:"" help:"Generate an EPC payment QR artifact from manual payment details."`
	Validate ValidateCmd `cmd:"" help:"Validate and normalize payment details without writing a QR artifact."`
	FromText FromTextCmd `cmd:"from-text" help:"Suggest payment details from copied invoice text."`
	FromPDF  FromPDFCmd  `cmd:"from-pdf" help:"Suggest payment details from a PDF through pdftotext."`
}

type PaymentDetailsFlags struct {
	Payee     string `help:"Payee name."`
	IBAN      string `help:"Payee IBAN."`
	Amount    string `help:"Payment amount in EUR."`
	Reference string `help:"Remittance reference."`
	BIC       string `help:"Optional payee BIC."`
}

func (flags PaymentDetailsFlags) paymentDetails() invoiceqr.PaymentDetails {
	return invoiceqr.PaymentDetails{
		Payee:     flags.Payee,
		IBAN:      flags.IBAN,
		Amount:    flags.Amount,
		Reference: flags.Reference,
		BIC:       flags.BIC,
	}
}

type QROutputFlags struct {
	Out    string `help:"Output path."`
	Format string `help:"Output format override."`
	Force  bool   `help:"Overwrite an existing output file."`
}

func (flags QROutputFlags) qrOutputOptions() invoiceqr.QROutputOptions {
	return invoiceqr.QROutputOptions{
		Out:    flags.Out,
		Format: flags.Format,
		Force:  flags.Force,
	}
}

type ConfirmationFlags struct {
	Yes bool `help:"Skip confirmation for manual payment details."`
}

type GenerateCmd struct {
	PaymentDetailsFlags `embed:""`
	QROutputFlags       `embed:""`
	ConfirmationFlags   `embed:""`
	DryRun              bool `help:"Validate and preflight without prompting or writing."`
	JSON                bool `help:"Print a machine-readable JSON envelope."`
}

func (cmd GenerateCmd) Run() error {
	if cmd.DryRun {
		if !cmd.JSON {
			return errors.New("dry-run: requires --json")
		}
		plan, err := invoiceqr.BuildPaymentArtifactPlan(invoiceqr.PaymentArtifactPlanOptions{
			Details: cmd.paymentDetails(),
			Output:  cmd.qrOutputOptions(),
		})
		if err != nil {
			return printJSONError("generation_error", err)
		}
		return printJSONSuccess(generateDryRunJSONData(plan))
	}
	if cmd.JSON {
		return cmd.runJSON()
	}
	return invoiceqr.GeneratePaymentArtifact(
		invoiceqr.PaymentGenerationOptions{
			Details:          cmd.paymentDetails(),
			Output:           cmd.qrOutputOptions(),
			SkipConfirmation: cmd.Yes,
		},
		confirmPaymentDetails,
	)
}

func (cmd GenerateCmd) runJSON() error {
	var confirm invoiceqr.PaymentConfirmationFunc
	if !cmd.Yes {
		confirm = func(details invoiceqr.ValidatedPaymentDetails) (bool, error) {
			return confirmPaymentDetailsWithInputOutput(details, os.Stdin, os.Stderr)
		}
	}
	result, err := invoiceqr.GeneratePaymentArtifactWithResult(
		invoiceqr.PaymentGenerationOptions{
			Details:          cmd.paymentDetails(),
			Output:           cmd.qrOutputOptions(),
			SkipConfirmation: cmd.Yes,
		},
		confirm,
	)
	if err != nil {
		return printJSONError("generation_error", err)
	}
	return printJSONSuccess(generateArtifactJSONData(result.Plan, result.Artifact))
}

type ValidateCmd struct {
	PaymentDetailsFlags `embed:""`
	JSON                bool `help:"Print a machine-readable JSON envelope."`
}

func (cmd ValidateCmd) Run() error {
	details, err := invoiceqr.ValidatePaymentDetails(cmd.paymentDetails())
	if err != nil {
		if cmd.JSON {
			return printJSONError("validation_error", err)
		}
		return err
	}
	if cmd.JSON {
		return printJSONSuccess(validateJSONData(details))
	}
	printPaymentDetails(details)
	return nil
}

type FromTextCmd struct {
	File                string `arg:"" optional:"" help:"Invoice text file. Reads stdin when omitted."`
	PaymentDetailsFlags `embed:""`
	QROutputFlags       `embed:""`
	DryRun              bool `help:"Suggest and preflight without prompting or writing. Requires --json."`
	JSON                bool `help:"Print a machine-readable JSON envelope. Requires --dry-run."`
}

func (cmd FromTextCmd) Run() error {
	if err := validateSuggestionJSONFlags(cmd.DryRun, cmd.JSON); err != nil {
		return err
	}
	text, err := readInvoiceText(cmd.File)
	if err != nil {
		if cmd.JSON {
			return printJSONError("input_error", err)
		}
		return err
	}
	if cmd.DryRun {
		return printSuggestionDryRunJSON(text, cmd.paymentDetails(), cmd.qrOutputOptions())
	}
	confirm := confirmPaymentDetails
	if cmd.File == "" {
		confirm = confirmPaymentDetailsFromTerminal
	}
	return generateSuggestedPaymentArtifact(text, cmd.paymentDetails(), cmd.qrOutputOptions(), confirm)
}

func generateSuggestedPaymentArtifact(text string, overrides invoiceqr.PaymentDetails, output invoiceqr.QROutputOptions, confirm invoiceqr.PaymentConfirmationFunc) error {
	suggestion, err := invoiceqr.SuggestPaymentDetailsFromText(text, overrides)
	if err != nil {
		return err
	}
	return invoiceqr.GeneratePaymentArtifact(
		invoiceqr.PaymentGenerationOptions{
			Details: invoiceqr.PaymentDetails{
				Payee:     suggestion.Payee,
				IBAN:      suggestion.IBAN,
				Amount:    suggestion.Amount,
				Reference: suggestion.Reference,
				BIC:       suggestion.BIC,
			},
			Output: output,
		},
		confirm,
	)
}

type FromPDFCmd struct {
	PDF                 string `arg:"" help:"Invoice PDF path."`
	PaymentDetailsFlags `embed:""`
	QROutputFlags       `embed:""`
	DryRun              bool `help:"Suggest and preflight without prompting or writing. Requires --json."`
	JSON                bool `help:"Print a machine-readable JSON envelope. Requires --dry-run."`
}

func (cmd FromPDFCmd) Run() error {
	if err := validateSuggestionJSONFlags(cmd.DryRun, cmd.JSON); err != nil {
		return err
	}
	text, err := extractPDFText(cmd.PDF, pdfTextCommandRunner)
	if err != nil {
		if cmd.JSON {
			return printJSONError("extraction_error", err)
		}
		return err
	}
	if cmd.DryRun {
		return printSuggestionDryRunJSON(text, cmd.paymentDetails(), cmd.qrOutputOptions())
	}
	return generateSuggestedPaymentArtifact(text, cmd.paymentDetails(), cmd.qrOutputOptions(), confirmPaymentDetails)
}

func validateSuggestionJSONFlags(dryRun, json bool) error {
	switch {
	case dryRun && !json:
		return errors.New("dry-run: requires --json")
	case json && !dryRun:
		return errors.New("json: requires --dry-run")
	default:
		return nil
	}
}

func printSuggestionDryRunJSON(text string, overrides invoiceqr.PaymentDetails, output invoiceqr.QROutputOptions) error {
	result, err := invoiceqr.BuildSuggestedPaymentArtifactPlan(invoiceqr.SuggestedPaymentArtifactPlanOptions{
		Text:      text,
		Overrides: overrides,
		Output:    output,
	})
	if err != nil {
		if !result.HasReport {
			return printJSONError("suggestion_error", err)
		}
		if printErr := printJSONEnvelope(suggestionDryRunJSONData(result), newCLIErrorJSON("generation_error", err)); printErr != nil {
			return printErr
		}
		return cliExitError{code: 1}
	}
	return printJSONSuccess(suggestionDryRunJSONData(result))
}

type commandRunner func(string, ...string) ([]byte, error)

var pdfTextCommandRunner commandRunner = runCommand

func runCommand(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func extractPDFText(path string, runner commandRunner) (string, error) {
	pdf := strings.TrimSpace(path)
	if pdf == "" {
		return "", errors.New("pdf: required")
	}
	output, err := runner("pdftotext", pdf, "-")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("pdftotext: not found; install poppler/pdftotext and retry")
		}
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return string(output), nil
}

func printPaymentDetails(details invoiceqr.ValidatedPaymentDetails) {
	printPaymentDetailsTo(os.Stdout, details)
}

func printPaymentDetailsTo(output io.Writer, details invoiceqr.ValidatedPaymentDetails) {
	fmt.Fprintln(output, "Payment Details")
	fmt.Fprintf(output, "Payee: %s\n", details.Payee)
	fmt.Fprintf(output, "IBAN: %s\n", details.IBAN)
	fmt.Fprintf(output, "Amount: EUR%s\n", details.Amount)
	if details.BIC != "" {
		fmt.Fprintf(output, "BIC: %s\n", details.BIC)
	}
	fmt.Fprintf(output, "Reference: %s\n", details.Reference.Value)
	fmt.Fprintf(output, "Reference Type: %s\n", referenceTypeLabel(details.Reference.Kind))
}

func referenceTypeLabel(kind invoiceqr.RemittanceKind) string {
	if kind == invoiceqr.StructuredReference {
		return "Belgian Structured Reference"
	}
	return "Unstructured Remittance Information"
}

func confirmPaymentDetails(details invoiceqr.ValidatedPaymentDetails) (bool, error) {
	return confirmPaymentDetailsWithInput(details, os.Stdin)
}

var terminalInputPath = defaultTerminalInputPath()

func confirmPaymentDetailsFromTerminal(details invoiceqr.ValidatedPaymentDetails) (bool, error) {
	terminal, err := os.Open(terminalInputPath)
	if err != nil {
		return false, err
	}
	defer terminal.Close()
	return confirmPaymentDetailsWithInput(details, terminal)
}

func confirmPaymentDetailsWithInput(details invoiceqr.ValidatedPaymentDetails, input io.Reader) (bool, error) {
	return confirmPaymentDetailsWithInputOutput(details, input, os.Stdout)
}

func confirmPaymentDetailsWithInputOutput(details invoiceqr.ValidatedPaymentDetails, input io.Reader, output io.Writer) (bool, error) {
	printPaymentDetailsTo(output, details)
	fmt.Fprint(output, "Write QR artifact? [y/N]: ")

	answer, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	if errors.Is(err, io.EOF) && answer == "" {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func readInvoiceText(path string) (string, error) {
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func main() {
	ctx := kong.Parse(&CLI{}, kong.Name("invoiceqr"), kong.Description("Generate Belgian-compatible SEPA/EPC payment QR codes."))
	err := ctx.Run()
	var exitErr cliExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.code)
	}
	ctx.FatalIfErrorf(err)
}

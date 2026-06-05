package main

import (
	"bufio"
	"encoding/json"
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
}

func (cmd GenerateCmd) Run() error {
	return invoiceqr.GeneratePaymentArtifact(
		invoiceqr.PaymentGenerationOptions{
			Details:          cmd.paymentDetails(),
			Output:           cmd.qrOutputOptions(),
			SkipConfirmation: cmd.Yes,
		},
		confirmPaymentDetails,
	)
}

type ValidateCmd struct {
	PaymentDetailsFlags `embed:""`
	JSON                bool `help:"Print a machine-readable JSON envelope."`
}

func (cmd ValidateCmd) Run() error {
	details, err := invoiceqr.ValidatePaymentDetails(cmd.paymentDetails())
	if err != nil {
		if cmd.JSON {
			if printErr := printJSONEnvelope(nil, newCLIErrorJSON("validation_error", err)); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if cmd.JSON {
		return printJSONEnvelope(validateJSONData(details), nil)
	}
	printPaymentDetails(details)
	return nil
}

type FromTextCmd struct {
	File                string `arg:"" optional:"" help:"Invoice text file. Reads stdin when omitted."`
	PaymentDetailsFlags `embed:""`
	QROutputFlags       `embed:""`
}

func (cmd FromTextCmd) Run() error {
	text, err := readInvoiceText(cmd.File)
	if err != nil {
		return err
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
			Details: suggestionPaymentDetails(suggestion),
			Output:  output,
		},
		confirm,
	)
}

type FromPDFCmd struct {
	PDF                 string `arg:"" help:"Invoice PDF path."`
	PaymentDetailsFlags `embed:""`
	QROutputFlags       `embed:""`
}

func (cmd FromPDFCmd) Run() error {
	text, err := extractPDFText(cmd.PDF, pdfTextCommandRunner)
	if err != nil {
		return err
	}
	return generateSuggestedPaymentArtifact(text, cmd.paymentDetails(), cmd.qrOutputOptions(), confirmPaymentDetails)
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
	fmt.Println("Payment Details")
	fmt.Printf("Payee: %s\n", details.Payee)
	fmt.Printf("IBAN: %s\n", details.IBAN)
	fmt.Printf("Amount: EUR%s\n", details.Amount)
	if details.BIC != "" {
		fmt.Printf("BIC: %s\n", details.BIC)
	}
	fmt.Printf("Reference: %s\n", details.Reference.Value)
	fmt.Printf("Reference Type: %s\n", referenceTypeLabel(details.Reference.Kind))
}

func referenceTypeLabel(kind invoiceqr.RemittanceKind) string {
	if kind == invoiceqr.StructuredReference {
		return "Belgian Structured Reference"
	}
	return "Unstructured Remittance Information"
}

type jsonEnvelope struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
	Error   any  `json:"error"`
}

type cliErrorJSON struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type paymentDetailsJSON struct {
	Payee     string        `json:"payee"`
	IBAN      string        `json:"iban"`
	Amount    string        `json:"amount"`
	BIC       string        `json:"bic"`
	Reference referenceJSON `json:"reference"`
}

type referenceJSON struct {
	Value string `json:"value"`
	Kind  string `json:"kind"`
}

func validateJSONData(details invoiceqr.ValidatedPaymentDetails) paymentDetailsJSON {
	return paymentDetailsJSON{
		Payee:  details.Payee,
		IBAN:   details.IBAN,
		Amount: details.Amount,
		BIC:    details.BIC,
		Reference: referenceJSON{
			Value: details.Reference.Value,
			Kind:  string(details.Reference.Kind),
		},
	}
}

func newCLIErrorJSON(code string, err error) cliErrorJSON {
	field := ""
	message := err.Error()
	if candidate, rest, ok := strings.Cut(message, ":"); ok && knownErrorField(candidate) {
		field = candidate
		message = strings.TrimSpace(rest)
	}
	return cliErrorJSON{
		Code:    code,
		Field:   field,
		Message: message,
	}
}

func knownErrorField(field string) bool {
	switch field {
	case "payee", "iban", "amount", "reference", "bic", "out", "format", "confirmation":
		return true
	default:
		return false
	}
}

func printJSONEnvelope(data any, errData any) error {
	envelope := jsonEnvelope{
		Success: errData == nil,
		Data:    data,
		Error:   errData,
	}
	return json.NewEncoder(os.Stdout).Encode(envelope)
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
	printPaymentDetails(details)
	fmt.Print("Write QR artifact? [y/N]: ")

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

func suggestionPaymentDetails(details invoiceqr.SuggestedPaymentDetails) invoiceqr.PaymentDetails {
	return invoiceqr.PaymentDetails{
		Payee:     details.Payee,
		IBAN:      details.IBAN,
		Amount:    details.Amount,
		Reference: details.Reference,
		BIC:       details.BIC,
	}
}

func main() {
	ctx := kong.Parse(&CLI{}, kong.Name("invoiceqr"), kong.Description("Generate Belgian-compatible SEPA/EPC payment QR codes."))
	ctx.FatalIfErrorf(ctx.Run())
}

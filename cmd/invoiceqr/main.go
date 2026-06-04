package main

import (
	"errors"

	"github.com/alecthomas/kong"
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

type QROutputFlags struct {
	Out    string `help:"Output path."`
	Format string `help:"Output format override."`
	Force  bool   `help:"Overwrite an existing output file."`
}

type ConfirmationFlags struct {
	Yes bool `help:"Skip confirmation for manual payment details."`
}

type GenerateCmd struct {
	PaymentDetailsFlags `embed:""`
	QROutputFlags       `embed:""`
	ConfirmationFlags   `embed:""`
}

func (GenerateCmd) Run() error {
	return errNotImplemented("generate")
}

type ValidateCmd struct {
	PaymentDetailsFlags `embed:""`
}

func (ValidateCmd) Run() error {
	return errNotImplemented("validate")
}

type FromTextCmd struct {
	File                string `arg:"" optional:"" help:"Invoice text file. Reads stdin when omitted."`
	PaymentDetailsFlags `embed:""`
	QROutputFlags       `embed:""`
}

func (FromTextCmd) Run() error {
	return errNotImplemented("from-text")
}

type FromPDFCmd struct {
	PDF                 string `arg:"" optional:"" help:"Invoice PDF path."`
	PaymentDetailsFlags `embed:""`
	QROutputFlags       `embed:""`
}

func (FromPDFCmd) Run() error {
	return errNotImplemented("from-pdf")
}

func errNotImplemented(command string) error {
	return errors.New(command + " is not implemented yet")
}

func main() {
	ctx := kong.Parse(&CLI{}, kong.Name("invoiceqr"), kong.Description("Generate Belgian-compatible SEPA/EPC payment QR codes."))
	ctx.FatalIfErrorf(ctx.Run())
}

package invoiceqr

import (
	"errors"
	"fmt"
)

type PaymentGenerationOptions struct {
	Details          PaymentDetails
	Output           QROutputOptions
	SkipConfirmation bool
}

type PaymentConfirmationFunc func(ValidatedPaymentDetails) (bool, error)
type QRArtifactWriteFunc func(string, QROutputOptions) error

func GeneratePaymentArtifact(options PaymentGenerationOptions, confirm PaymentConfirmationFunc) error {
	return generatePaymentArtifact(options, confirm, WriteQRArtifact)
}

func generatePaymentArtifact(options PaymentGenerationOptions, confirm PaymentConfirmationFunc, write QRArtifactWriteFunc) error {
	validated, err := ValidatePaymentDetails(options.Details)
	if err != nil {
		return err
	}

	if !options.SkipConfirmation {
		if confirm == nil {
			return errors.New("confirmation: required")
		}
		confirmed, err := confirm(validated)
		if err != nil {
			return fmt.Errorf("confirmation: %w", err)
		}
		if !confirmed {
			return errors.New("confirmation: refused")
		}
	}

	payload, err := BuildEPCPayload(confirmedPaymentDetails(validated))
	if err != nil {
		return err
	}
	return write(payload, options.Output)
}

func confirmedPaymentDetails(details ValidatedPaymentDetails) ConfirmedPaymentDetails {
	return ConfirmedPaymentDetails{
		Payee:     details.Payee,
		IBAN:      details.IBAN,
		Amount:    details.Amount,
		Reference: details.Reference,
		BIC:       details.BIC,
	}
}

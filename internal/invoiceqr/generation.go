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

type PaymentArtifactPlanOptions struct {
	Details PaymentDetails
	Output  QROutputOptions
}

type PaymentArtifactPlan struct {
	Details       ValidatedPaymentDetails
	ReferenceKind RemittanceKind
	EPC           EPCPayloadData
	Output        QROutputPreflight
}

type EPCPayloadData struct {
	ServiceTag     string
	Version        string
	CharacterSet   string
	Identification string
	Currency       string
	Payload        string
}

type PaymentConfirmationFunc func(ValidatedPaymentDetails) (bool, error)
type QRArtifactWriteFunc func(string, QROutputPreflight) error

func GeneratePaymentArtifact(options PaymentGenerationOptions, confirm PaymentConfirmationFunc) error {
	return generatePaymentArtifact(options, confirm, WritePlannedQRArtifact)
}

func BuildPaymentArtifactPlan(options PaymentArtifactPlanOptions) (PaymentArtifactPlan, error) {
	return buildPaymentArtifactPlan(options, PreflightQROutput)
}

type qrOutputPreflightFunc func(QROutputOptions) (QROutputPreflight, error)

func buildPaymentArtifactPlan(options PaymentArtifactPlanOptions, preflight qrOutputPreflightFunc) (PaymentArtifactPlan, error) {
	validated, err := ValidatePaymentDetails(options.Details)
	if err != nil {
		return PaymentArtifactPlan{}, err
	}

	payload, err := BuildEPCPayload(confirmedPaymentDetails(validated))
	if err != nil {
		return PaymentArtifactPlan{}, err
	}

	output, err := preflight(options.Output)
	if err != nil {
		return PaymentArtifactPlan{}, err
	}

	return PaymentArtifactPlan{
		Details:       validated,
		ReferenceKind: validated.Reference.Kind,
		EPC: EPCPayloadData{
			ServiceTag:     "BCD",
			Version:        "002",
			CharacterSet:   "1",
			Identification: "SCT",
			Currency:       "EUR",
			Payload:        payload,
		},
		Output: output,
	}, nil
}

func generatePaymentArtifact(options PaymentGenerationOptions, confirm PaymentConfirmationFunc, write QRArtifactWriteFunc) error {
	plan, err := BuildPaymentArtifactPlan(PaymentArtifactPlanOptions{
		Details: options.Details,
		Output:  options.Output,
	})
	if err != nil {
		return err
	}

	if !options.SkipConfirmation {
		if confirm == nil {
			return errors.New("confirmation: required")
		}
		confirmed, err := confirm(plan.Details)
		if err != nil {
			return fmt.Errorf("confirmation: %w", err)
		}
		if !confirmed {
			return errors.New("confirmation: refused")
		}
	}

	return write(plan.EPC.Payload, plan.Output)
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

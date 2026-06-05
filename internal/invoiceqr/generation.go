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

type PaymentArtifactResult struct {
	Plan     PaymentArtifactPlan
	Artifact QRArtifactWriteResult
}

type PaymentConfirmationFunc func(ValidatedPaymentDetails) (bool, error)
type QRArtifactWriteFunc func(string, QROutputPreflight) error
type QRArtifactWriteResultFunc func(string, QROutputPreflight) (QRArtifactWriteResult, error)

func GeneratePaymentArtifact(options PaymentGenerationOptions, confirm PaymentConfirmationFunc) error {
	return generatePaymentArtifact(options, confirm, WritePlannedQRArtifact)
}

func GeneratePaymentArtifactWithResult(options PaymentGenerationOptions, confirm PaymentConfirmationFunc) (PaymentArtifactResult, error) {
	return generatePaymentArtifactWithResult(options, confirm, WritePlannedQRArtifactWithResult)
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

	epc, err := BuildEPCPayloadData(confirmedPaymentDetails(validated))
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
		EPC:           epc,
		Output:        output,
	}, nil
}

func generatePaymentArtifact(options PaymentGenerationOptions, confirm PaymentConfirmationFunc, write QRArtifactWriteFunc) error {
	_, err := generatePaymentArtifactWithResult(
		options,
		confirm,
		func(payload string, output QROutputPreflight) (QRArtifactWriteResult, error) {
			if err := write(payload, output); err != nil {
				return QRArtifactWriteResult{}, err
			}
			return QRArtifactWriteResult{}, nil
		},
	)
	return err
}

func generatePaymentArtifactWithResult(options PaymentGenerationOptions, confirm PaymentConfirmationFunc, write QRArtifactWriteResultFunc) (PaymentArtifactResult, error) {
	plan, err := BuildPaymentArtifactPlan(PaymentArtifactPlanOptions{
		Details: options.Details,
		Output:  options.Output,
	})
	if err != nil {
		return PaymentArtifactResult{}, err
	}

	if !options.SkipConfirmation {
		if confirm == nil {
			return PaymentArtifactResult{}, errors.New("confirmation: required")
		}
		confirmed, err := confirm(plan.Details)
		if err != nil {
			return PaymentArtifactResult{}, fmt.Errorf("confirmation: %w", err)
		}
		if !confirmed {
			return PaymentArtifactResult{}, errors.New("confirmation: refused")
		}
	}

	artifact, err := write(plan.EPC.Payload, plan.Output)
	if err != nil {
		return PaymentArtifactResult{}, err
	}
	return PaymentArtifactResult{
		Plan:     plan,
		Artifact: artifact,
	}, nil
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

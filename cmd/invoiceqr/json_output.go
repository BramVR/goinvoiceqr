package main

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/bramvr/goinvoiceqr/internal/invoiceqr"
)

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
	BIC       string        `json:"bic,omitempty"`
	Reference referenceJSON `json:"reference"`
}

type cliExitError struct {
	code int
}

func (err cliExitError) Error() string {
	return "exit"
}

type referenceJSON struct {
	Value string `json:"value"`
	Kind  string `json:"kind"`
}

type generateDryRunJSON struct {
	PaymentDetails paymentDetailsJSON `json:"payment_details"`
	EPC            epcJSON            `json:"epc"`
	Output         qrOutputJSON       `json:"output"`
}

type generateArtifactJSON struct {
	PaymentDetails paymentDetailsJSON `json:"payment_details"`
	EPC            epcJSON            `json:"epc"`
	Output         artifactOutputJSON `json:"output"`
}

type suggestionDryRunJSON struct {
	Suggestions suggestionFieldsJSON `json:"suggestions"`
	Plan        *generateDryRunJSON  `json:"plan,omitempty"`
}

type suggestionFieldsJSON struct {
	Payee     suggestionFieldJSON  `json:"payee"`
	IBAN      suggestionFieldJSON  `json:"iban"`
	Amount    suggestionFieldJSON  `json:"amount"`
	Reference suggestionFieldJSON  `json:"reference"`
	BIC       *suggestionFieldJSON `json:"bic,omitempty"`
}

type suggestionFieldJSON struct {
	Value    string `json:"value"`
	Source   string `json:"source"`
	Evidence string `json:"evidence,omitempty"`
}

type epcJSON struct {
	ServiceTag     string `json:"service_tag"`
	Version        string `json:"version"`
	CharacterSet   string `json:"character_set"`
	Identification string `json:"identification"`
	Currency       string `json:"currency"`
	Payload        string `json:"payload,omitempty"`
}

type qrOutputJSON struct {
	Path          string `json:"path"`
	Format        string `json:"format"`
	Force         bool   `json:"force"`
	Exists        bool   `json:"exists"`
	IsSymlink     bool   `json:"is_symlink"`
	WillOverwrite bool   `json:"will_overwrite"`
}

type artifactOutputJSON struct {
	Path      string `json:"path"`
	Format    string `json:"format"`
	ByteCount int    `json:"byte_count"`
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

func generateDryRunJSONData(plan invoiceqr.PaymentArtifactPlan) generateDryRunJSON {
	return generateDryRunJSON{
		PaymentDetails: validateJSONData(plan.Details),
		EPC: epcJSON{
			ServiceTag:     plan.EPC.ServiceTag,
			Version:        plan.EPC.Version,
			CharacterSet:   plan.EPC.CharacterSet,
			Identification: plan.EPC.Identification,
			Currency:       plan.EPC.Currency,
			Payload:        plan.EPC.Payload,
		},
		Output: qrOutputJSON{
			Path:          plan.Output.Path,
			Format:        string(plan.Output.Format),
			Force:         plan.Output.Force,
			Exists:        plan.Output.Exists,
			IsSymlink:     plan.Output.IsSymlink,
			WillOverwrite: plan.Output.WillOverwrite,
		},
	}
}

func generateArtifactJSONData(plan invoiceqr.PaymentArtifactPlan, result invoiceqr.QRArtifactWriteResult) generateArtifactJSON {
	return generateArtifactJSON{
		PaymentDetails: validateJSONData(plan.Details),
		EPC: epcJSON{
			ServiceTag:     plan.EPC.ServiceTag,
			Version:        plan.EPC.Version,
			CharacterSet:   plan.EPC.CharacterSet,
			Identification: plan.EPC.Identification,
			Currency:       plan.EPC.Currency,
		},
		Output: artifactOutputJSON{
			Path:      result.Path,
			Format:    string(result.Format),
			ByteCount: result.ByteCount,
		},
	}
}

func suggestionDryRunJSONData(report invoiceqr.SuggestedPaymentDetailsReport, plan invoiceqr.PaymentArtifactPlan) suggestionDryRunJSON {
	planData := generateDryRunJSONData(plan)
	return suggestionDryRunJSON{
		Suggestions: suggestionFieldsJSONData(report),
		Plan:        &planData,
	}
}

func suggestionDryRunErrorJSONData(report invoiceqr.SuggestedPaymentDetailsReport) suggestionDryRunJSON {
	return suggestionDryRunJSON{
		Suggestions: suggestionFieldsJSONData(report),
	}
}

func suggestionFieldsJSONData(report invoiceqr.SuggestedPaymentDetailsReport) suggestionFieldsJSON {
	var bic *suggestionFieldJSON
	if report.BIC.Value != "" {
		bicData := suggestionFieldJSONData(report.BIC)
		bic = &bicData
	}
	return suggestionFieldsJSON{
		Payee:     suggestionFieldJSONData(report.Payee),
		IBAN:      suggestionFieldJSONData(report.IBAN),
		Amount:    suggestionFieldJSONData(report.Amount),
		Reference: suggestionFieldJSONData(report.Reference),
		BIC:       bic,
	}
}

func suggestionFieldJSONData(field invoiceqr.SuggestedPaymentField) suggestionFieldJSON {
	return suggestionFieldJSON{
		Value:    field.Value,
		Source:   field.Source,
		Evidence: field.Evidence,
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

func printJSONSuccess(data any) error {
	return printJSONEnvelope(data, nil)
}

func printJSONError(code string, err error) error {
	if printErr := printJSONEnvelope(nil, newCLIErrorJSON(code, err)); printErr != nil {
		return printErr
	}
	return cliExitError{code: 1}
}

func printJSONEnvelope(data any, errData any) error {
	envelope := jsonEnvelope{
		Success: errData == nil,
		Data:    data,
		Error:   errData,
	}
	return json.NewEncoder(os.Stdout).Encode(envelope)
}

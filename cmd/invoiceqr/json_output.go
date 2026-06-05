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

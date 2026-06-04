package invoiceqr

import (
	"errors"
	"strings"
)

type ConfirmedPaymentDetails struct {
	Payee     string
	IBAN      string
	Amount    string
	Reference RemittanceReference
	BIC       string
}

func BuildEPCPayload(details ConfirmedPaymentDetails) (string, error) {
	structuredReference := ""
	unstructuredReference := ""
	switch details.Reference.Kind {
	case StructuredReference:
		structuredReference = details.Reference.Value
	case UnstructuredReference:
		unstructuredReference = details.Reference.Value
	default:
		return "", errors.New("reference: unknown remittance kind")
	}

	return strings.Join([]string{
		"BCD",
		"002",
		"1",
		"SCT",
		details.BIC,
		details.Payee,
		details.IBAN,
		"EUR" + details.Amount,
		"",
		structuredReference,
		unstructuredReference,
		"",
	}, "\n"), nil
}

package invoiceqr

import (
	"errors"
	"strings"
)

const (
	epcServiceTag     = "BCD"
	epcVersion        = "002"
	epcCharacterSet   = "1"
	epcIdentification = "SCT"
	epcCurrency       = "EUR"
)

type ConfirmedPaymentDetails struct {
	Payee     string
	IBAN      string
	Amount    string
	Reference RemittanceReference
	BIC       string
}

type EPCPayloadData struct {
	ServiceTag     string
	Version        string
	CharacterSet   string
	Identification string
	Currency       string
	Payload        string
}

func BuildEPCPayload(details ConfirmedPaymentDetails) (string, error) {
	data, err := BuildEPCPayloadData(details)
	if err != nil {
		return "", err
	}
	return data.Payload, nil
}

func BuildEPCPayloadData(details ConfirmedPaymentDetails) (EPCPayloadData, error) {
	structuredReference := ""
	unstructuredReference := ""
	switch details.Reference.Kind {
	case StructuredReference:
		structuredReference = details.Reference.Value
	case UnstructuredReference:
		unstructuredReference = details.Reference.Value
	default:
		return EPCPayloadData{}, errors.New("reference: unknown remittance kind")
	}

	payload := strings.Join([]string{
		epcServiceTag,
		epcVersion,
		epcCharacterSet,
		epcIdentification,
		details.BIC,
		details.Payee,
		details.IBAN,
		epcCurrency + details.Amount,
		"",
		structuredReference,
		unstructuredReference,
		"",
	}, "\n")

	return EPCPayloadData{
		ServiceTag:     epcServiceTag,
		Version:        epcVersion,
		CharacterSet:   epcCharacterSet,
		Identification: epcIdentification,
		Currency:       epcCurrency,
		Payload:        payload,
	}, nil
}

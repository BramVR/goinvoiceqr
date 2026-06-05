package invoiceqr

import (
	"fmt"
	"strings"
)

type SuggestedPaymentDetails struct {
	Payee     string
	IBAN      string
	Amount    string
	Reference string
	BIC       string
}

func SuggestPaymentDetailsFromText(text string, overrides PaymentDetails) (SuggestedPaymentDetails, error) {
	payee, err := chooseField("payee", overrides.Payee, findPayeeCandidates(text), false)
	if err != nil {
		return SuggestedPaymentDetails{}, err
	}
	iban, err := chooseField("iban", overrides.IBAN, findIBANCandidates(text), true)
	if err != nil {
		return SuggestedPaymentDetails{}, err
	}
	amount, err := chooseField("amount", overrides.Amount, findAmountCandidates(text), true)
	if err != nil {
		return SuggestedPaymentDetails{}, err
	}
	reference, err := chooseField("reference", overrides.Reference, findReferenceCandidates(text), false)
	if err != nil {
		return SuggestedPaymentDetails{}, err
	}

	return SuggestedPaymentDetails{
		Payee:     payee,
		IBAN:      iban,
		Amount:    amount,
		Reference: reference,
		BIC:       strings.TrimSpace(overrides.BIC),
	}, nil
}

func chooseField(name, override string, candidates []string, ambiguous bool) (string, error) {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override), nil
	}
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("%s: required", name)
	case 1:
		return candidates[0], nil
	default:
		if ambiguous {
			return "", fmt.Errorf("%s: ambiguous", name)
		}
		return candidates[0], nil
	}
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

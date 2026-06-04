package invoiceqr

import (
	"fmt"
	"regexp"
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
	payee, err := chooseField("payee", overrides.Payee, findLabeledLine(text, payeeLinePattern), false)
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

var (
	payeeLinePattern     = regexp.MustCompile(`(?im)^\s*(?:payee|beneficiary|supplier|name|begunstigde|leverancier)\s*:\s*(.+?)\s*$`)
	referenceLinePattern = regexp.MustCompile(`(?im)^\s*(?:reference|communication|remittance|mededeling|invoice)\s*:\s*(.+?)\s*$`)
	ibanCandidatePattern = regexp.MustCompile(`(?i)\b[A-Z]{2}[ \t]*[0-9]{2}(?:[ \t]*[A-Z0-9]){10,30}\b`)
	amountLinePattern    = regexp.MustCompile(`(?im)\b(?:amount|total|bedrag|totaal|montant)\b[^\n0-9-]*([0-9]+(?:[,.][0-9]{1,2})?)`)
	structuredRefPattern = regexp.MustCompile(`\+\+\+/?\d{3}/\d{4}/\d{5}\+\+\+`)
)

func chooseField(name, override string, candidates []string, ambiguous bool) (string, error) {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override), nil
	}
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("%s: missing", name)
	case 1:
		return candidates[0], nil
	default:
		if ambiguous {
			return "", fmt.Errorf("%s: ambiguous", name)
		}
		return candidates[0], nil
	}
}

func findLabeledLine(text string, pattern *regexp.Regexp) []string {
	matches := pattern.FindAllStringSubmatch(text, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = appendUnique(values, strings.TrimSpace(match[1]))
	}
	return values
}

func findIBANCandidates(text string) []string {
	matches := ibanCandidatePattern.FindAllString(text, -1)
	values := []string{}
	seen := map[string]bool{}
	for _, match := range matches {
		candidate := strings.TrimSpace(match)
		normalized, err := normalizeIBAN(candidate)
		if err != nil {
			continue
		}
		if !seen[normalized] {
			seen[normalized] = true
			values = append(values, candidate)
		}
	}
	return values
}

func findAmountCandidates(text string) []string {
	matches := amountLinePattern.FindAllStringSubmatch(text, -1)
	values := []string{}
	seen := map[string]bool{}
	for _, match := range matches {
		candidate := strings.TrimSpace(match[1])
		normalized, err := normalizeAmount(candidate)
		if err != nil {
			continue
		}
		if !seen[normalized] {
			seen[normalized] = true
			values = append(values, candidate)
		}
	}
	return values
}

func findReferenceCandidates(text string) []string {
	if match := structuredRefPattern.FindString(text); match != "" {
		return []string{match}
	}
	references := findLabeledLine(text, referenceLinePattern)
	if len(references) == 0 {
		return references
	}
	clean := make([]string, 0, len(references))
	for _, reference := range references {
		clean = appendUnique(clean, reference)
	}
	return clean
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

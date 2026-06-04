package invoiceqr

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
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
	payeeLinePattern            = regexp.MustCompile(`(?im)^\s*(?:payee|beneficiary|supplier|name|begunstigde|leverancier)\s*:\s*(.+?)\s*$`)
	referenceLinePattern        = regexp.MustCompile(`(?im)^\s*(?:reference|communication|remittance|mededeling|invoice)\s*:\s*(.+?)\s*$`)
	ibanCandidatePattern        = regexp.MustCompile(`(?i)\b[A-Z]{2}[ \t]*[0-9]{2}(?:[ \t]*[A-Z0-9]){10,30}\b`)
	amountLinePattern           = regexp.MustCompile(`(?im)^\s*(?:amount|total|bedrag|totaal|montant)\b[^\n]*`)
	amountTokenPattern          = regexp.MustCompile(amountTokenPatternText)
	currencyBeforeAmountPattern = regexp.MustCompile(`(?i)(?:EUR|€)\s*(` + amountTokenPatternText + `)`)
	currencyAfterAmountPattern  = regexp.MustCompile(`(?i)(?:^|[^0-9.,\p{Zs}\t-])\s*(` + amountTokenPatternText + `)\s*(?:EUR|€)`)
	signedAmountPattern         = regexp.MustCompile(`(?i)(?:EUR|€)\s*-\s*` + amountTokenPatternText + `|(?:^|[:\p{Zs}\t])-\s*` + amountTokenPatternText + `|(?:EUR|€)\s*\(\s*` + amountTokenPatternText + `\s*\)|(?:^|[:\p{Zs}\t])\(\s*(?:EUR|€)?\s*` + amountTokenPatternText + `\s*\)`)
	structuredRefPattern        = regexp.MustCompile(`\+\+\+/?\d{3}/\d{4}/\d{5}\+\+\+`)
)

const amountTokenPatternText = `[0-9](?:[0-9.,\p{Zs}\t]*[0-9])?`

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
	lines := amountLinePattern.FindAllString(text, -1)
	values := []string{}
	seen := map[string]bool{}
	for _, line := range lines {
		for _, candidate := range findAmountCandidatesInLine(line) {
			normalized, err := normalizeSuggestedAmount(candidate)
			if err != nil {
				continue
			}
			if !seen[normalized] {
				seen[normalized] = true
				values = append(values, normalized)
			}
		}
	}
	return values
}

func findAmountCandidatesInLine(line string) []string {
	if signedAmountPattern.MatchString(line) {
		return nil
	}
	currencyMatches := currencyBeforeAmountPattern.FindAllStringSubmatch(line, -1)
	candidates := []string{}
	for _, match := range currencyMatches {
		if match[1] != "" {
			candidates = append(candidates, match[1])
		}
	}
	if len(candidates) > 0 {
		return candidates
	}
	currencyMatches = currencyAfterAmountPattern.FindAllStringSubmatch(line, -1)
	for _, match := range currencyMatches {
		if match[1] != "" {
			candidates = append(candidates, match[1])
		}
	}
	if len(candidates) > 0 {
		return candidates
	}
	return amountTokenPattern.FindAllString(line, -1)
}

func normalizeSuggestedAmount(input string) (string, error) {
	amount := normalizeAmountWhitespace(input)
	lastDot := strings.LastIndex(amount, ".")
	lastComma := strings.LastIndex(amount, ",")

	switch {
	case lastDot >= 0 && lastComma >= 0:
		thousandsSeparator := ","
		decimalIndex := lastDot
		if lastComma > lastDot {
			thousandsSeparator = "."
			decimalIndex = lastComma
		}
		integer := amount[:decimalIndex]
		fraction := amount[decimalIndex+1:]
		if !validFraction(fraction) {
			return "", fmt.Errorf("malformed")
		}
		integer, err := normalizeSuggestedInteger(integer, thousandsSeparator)
		if err != nil {
			return "", err
		}
		return normalizeAmount(integer + "." + fraction)
	case lastDot >= 0:
		return normalizeSingleSeparatorAmount(amount, ".")
	case lastComma >= 0:
		return normalizeSingleSeparatorAmount(amount, ",")
	case strings.Contains(amount, " "):
		if !validGroupedInteger(amount, " ") {
			return "", fmt.Errorf("malformed")
		}
		return normalizeAmount(strings.ReplaceAll(amount, " ", ""))
	default:
		return normalizeAmount(amount)
	}
}

func normalizeSingleSeparatorAmount(amount, separator string) (string, error) {
	parts := strings.Split(amount, separator)
	if len(parts) == 2 && validFraction(parts[1]) {
		integer, err := normalizeSuggestedInteger(parts[0], "")
		if err != nil {
			return "", err
		}
		return normalizeAmount(integer + "." + parts[1])
	}
	if validGroupedInteger(amount, separator) {
		return normalizeAmount(strings.ReplaceAll(amount, separator, ""))
	}
	return "", fmt.Errorf("malformed")
}

func normalizeSuggestedInteger(integer, punctuationThousandsSeparator string) (string, error) {
	if strings.Contains(integer, " ") {
		if punctuationThousandsSeparator != "" && strings.Contains(integer, punctuationThousandsSeparator) {
			return "", fmt.Errorf("malformed")
		}
		if !validGroupedInteger(integer, " ") {
			return "", fmt.Errorf("malformed")
		}
		integer = strings.ReplaceAll(integer, " ", "")
	}
	if punctuationThousandsSeparator != "" && strings.Contains(integer, punctuationThousandsSeparator) {
		if !validGroupedInteger(integer, punctuationThousandsSeparator) {
			return "", fmt.Errorf("malformed")
		}
		integer = strings.ReplaceAll(integer, punctuationThousandsSeparator, "")
	}
	if !asciiDigits(integer) {
		return "", fmt.Errorf("malformed")
	}
	return integer, nil
}

func normalizeAmountWhitespace(input string) string {
	return strings.Join(strings.FieldsFunc(strings.TrimSpace(input), unicode.IsSpace), " ")
}

func validFraction(value string) bool {
	return len(value) >= 1 && len(value) <= 2 && asciiDigits(value)
}

func validGroupedInteger(value, separator string) bool {
	parts := strings.Split(value, separator)
	if len(parts) < 2 || len(parts[0]) < 1 || len(parts[0]) > 3 || !asciiDigits(parts[0]) {
		return false
	}
	for _, part := range parts[1:] {
		if len(part) != 3 || !asciiDigits(part) {
			return false
		}
	}
	return true
}

func asciiDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !asciiDigit(r) {
			return false
		}
	}
	return true
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

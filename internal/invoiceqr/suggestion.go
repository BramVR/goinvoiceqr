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

var (
	payeeLinePattern                      = regexp.MustCompile(`(?im)^\s*(?:payee|beneficiary|supplier|name|begunstigde|leverancier)\s*:\s*(.+?)\s*$`)
	creditorIBANLinePattern               = regexp.MustCompile(`(?im)^\s*(?:(?:creditor|payee|beneficiary|supplier|begunstigde|leverancier)\s*:\s*)?([^-–—\n]+?\b(?:BV|B\.V\.|NV|N\.V\.|BVBA|GmbH|SARL|S\.A\.|SA|Ltd|Limited|Inc|LLC|VZW|ASBL))(?:\s|[-–—]|$)[^\n]*\bIBAN\b`)
	referenceLinePattern                  = regexp.MustCompile(`(?im)^\s*(?:reference|communication|remittance|mededeling|invoice)\s*:\s*(.+?)\s*$`)
	ibanCandidatePattern                  = regexp.MustCompile(`(?i)\b[A-Z]{2}[ \t]*[0-9]{2}(?:[ \t]*[A-Z0-9]){10,30}\b`)
	amountDueLinePattern                  = regexp.MustCompile(`(?i)^\s*(?:total amount to pay|amount to pay|totaal te betalen|te betalen bedrag)\b[^\n]*`)
	amountLinePattern                     = regexp.MustCompile(`(?im)^\s*(?:amount|total|bedrag|totaal|montant)\b[^\n]*`)
	amountOnlyPattern                     = regexp.MustCompile(`^\s*` + amountTokenPatternText + `\s*$`)
	amountTokenPattern                    = regexp.MustCompile(amountTokenPatternText)
	currencyBeforeAmountPattern           = regexp.MustCompile(`(?i)(?:EUR|€)` + currencyWhitespacePatternText + `(` + amountTokenPatternText + `)`)
	currencyAfterAmountPattern            = regexp.MustCompile(`(?i)(?:^|[^0-9.,\p{Zs}\t-])` + currencyWhitespacePatternText + `(` + amountTokenPatternText + `)` + currencyWhitespacePatternText + `(?:EUR|€)`)
	standaloneCurrencyBeforeAmountPattern = regexp.MustCompile(`(?i)^` + currencyWhitespacePatternText + `(?:EUR|€)` + currencyWhitespacePatternText + `(` + amountTokenPatternText + `)` + currencyWhitespacePatternText + `$`)
	standaloneCurrencyAfterAmountPattern  = regexp.MustCompile(`(?i)^` + currencyWhitespacePatternText + `(` + amountTokenPatternText + `)` + currencyWhitespacePatternText + `(?:EUR|€)` + currencyWhitespacePatternText + `$`)
	signedAmountPattern                   = regexp.MustCompile(`(?i)(?:EUR|€)` + currencyWhitespacePatternText + `-` + currencyWhitespacePatternText + amountTokenPatternText + `|(?:^|[:\p{Zs}\t])-` + currencyWhitespacePatternText + amountTokenPatternText + `|(?:EUR|€)` + currencyWhitespacePatternText + `\(` + currencyWhitespacePatternText + amountTokenPatternText + currencyWhitespacePatternText + `\)|(?:^|[:\p{Zs}\t])\(` + currencyWhitespacePatternText + `(?:EUR|€)?` + currencyWhitespacePatternText + amountTokenPatternText + currencyWhitespacePatternText + `\)`)
	structuredRefPattern                  = regexp.MustCompile(`\+\+\+/?\d{3}/\d{4}/\d{5}\+\+\+`)
)

const amountTokenPatternText = `[0-9](?:[0-9.,\p{Zs}\t]*[0-9])?`
const currencyWhitespacePatternText = `[\s\p{Zs}]*`

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

func findLabeledLine(text string, pattern *regexp.Regexp) []string {
	matches := pattern.FindAllStringSubmatch(text, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = appendUnique(values, strings.TrimSpace(match[1]))
	}
	return values
}

func findPayeeCandidates(text string) []string {
	explicit := []string{}
	inferred := []string{}
	for _, line := range strings.Split(text, "\n") {
		if match := payeeLinePattern.FindStringSubmatch(line); len(match) > 1 {
			if creditorMatch := creditorIBANLinePattern.FindStringSubmatch(line); len(creditorMatch) > 1 {
				explicit = appendUnique(explicit, strings.TrimSpace(creditorMatch[1]))
				continue
			}
			explicit = appendUnique(explicit, strings.TrimSpace(match[1]))
			continue
		}
		if match := creditorIBANLinePattern.FindStringSubmatch(line); len(match) > 1 {
			inferred = appendUnique(inferred, strings.TrimSpace(match[1]))
		}
	}
	return append(explicit, inferred...)
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
	if values := findAmountCandidatesNearLines(text, amountDueLinePattern, true); len(values) > 0 {
		return values
	}
	return findAmountCandidatesNearLines(text, amountLinePattern, false)
}

func findAmountCandidatesNearLines(text string, pattern *regexp.Regexp, allowNextLine bool) []string {
	lines := strings.Split(text, "\n")
	values := []string{}
	seen := map[string]bool{}
	for index, line := range lines {
		if !pattern.MatchString(line) {
			continue
		}
		candidates := findAmountCandidatesInLine(line)
		if allowNextLine {
			candidates = findPreferredAmountCandidatesInLine(line)
			if len(candidates) == 0 {
				candidates = findAmountCandidatesOnNextNonEmptyLine(lines, index)
			}
		}
		for _, candidate := range candidates {
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

func findAmountCandidatesOnNextNonEmptyLine(lines []string, index int) []string {
	for _, line := range lines[index+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		return findStandaloneCurrencyAmountCandidatesInLine(line)
	}
	return nil
}

func findPreferredAmountCandidatesInLine(line string) []string {
	if candidates := findCurrencyAmountCandidatesInLine(line); len(candidates) > 0 {
		return candidates
	}
	_, value, ok := strings.Cut(line, ":")
	if !ok {
		return nil
	}
	if candidates := findCurrencyAmountCandidatesInLine(value); len(candidates) > 0 {
		return candidates
	}
	if amountOnlyPattern.MatchString(value) {
		return []string{strings.TrimSpace(value)}
	}
	return nil
}

func findAmountCandidatesInLine(line string) []string {
	if signedAmountPattern.MatchString(line) {
		return nil
	}
	if candidates := findCurrencyAmountCandidatesInLine(line); len(candidates) > 0 {
		return candidates
	}
	return amountTokenPattern.FindAllString(line, -1)
}

func findCurrencyAmountCandidatesInLine(line string) []string {
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
	return nil
}

func findStandaloneCurrencyAmountCandidatesInLine(line string) []string {
	if signedAmountPattern.MatchString(line) {
		return nil
	}
	candidates := []string{}
	for _, match := range standaloneCurrencyBeforeAmountPattern.FindAllStringSubmatch(line, -1) {
		if match[1] != "" {
			candidates = append(candidates, match[1])
		}
	}
	if len(candidates) > 0 {
		return candidates
	}
	for _, match := range standaloneCurrencyAfterAmountPattern.FindAllStringSubmatch(line, -1) {
		if match[1] != "" {
			candidates = append(candidates, match[1])
		}
	}
	return candidates
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

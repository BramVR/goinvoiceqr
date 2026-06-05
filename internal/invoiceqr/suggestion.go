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

type SuggestedPaymentField struct {
	Value    string
	Source   string
	Evidence string
}

type SuggestedPaymentDetailsReport struct {
	Payee        SuggestedPaymentField
	IBAN         SuggestedPaymentField
	Amount       SuggestedPaymentField
	Reference    SuggestedPaymentField
	BIC          SuggestedPaymentField
	AgentContext AgentContext
	Details      SuggestedPaymentDetails
}

type SuggestedPaymentArtifactPlanOptions struct {
	Text      string
	Overrides PaymentDetails
	Output    QROutputOptions
}

type SuggestedPaymentArtifactPlan struct {
	Report    SuggestedPaymentDetailsReport
	HasReport bool
	Plan      PaymentArtifactPlan
	HasPlan   bool
}

func SuggestPaymentDetailsFromText(text string, overrides PaymentDetails) (SuggestedPaymentDetails, error) {
	report, err := SuggestPaymentDetailsReportFromText(text, overrides)
	if err != nil {
		return SuggestedPaymentDetails{}, err
	}
	return report.Details, nil
}

func BuildSuggestedPaymentArtifactPlan(options SuggestedPaymentArtifactPlanOptions) (SuggestedPaymentArtifactPlan, error) {
	report, err := SuggestPaymentDetailsReportFromText(options.Text, options.Overrides)
	if err != nil {
		return SuggestedPaymentArtifactPlan{}, err
	}
	plan, err := BuildPaymentArtifactPlan(PaymentArtifactPlanOptions{
		Details: suggestedPaymentDetails(report.Details),
		Output:  options.Output,
	})
	if err != nil {
		return SuggestedPaymentArtifactPlan{Report: report, HasReport: true}, err
	}
	return SuggestedPaymentArtifactPlan{
		Report:    report,
		HasReport: true,
		Plan:      plan,
		HasPlan:   true,
	}, nil
}

func SuggestPaymentDetailsReportFromText(text string, overrides PaymentDetails) (SuggestedPaymentDetailsReport, error) {
	payee, err := chooseField("payee", overrides.Payee, findPayeeCandidates(text), false)
	if err != nil {
		return SuggestedPaymentDetailsReport{}, err
	}
	iban, err := chooseField("iban", overrides.IBAN, findIBANCandidates(text), true)
	if err != nil {
		return SuggestedPaymentDetailsReport{}, err
	}
	amount, err := chooseField("amount", overrides.Amount, findAmountCandidates(text), true)
	if err != nil {
		return SuggestedPaymentDetailsReport{}, err
	}
	reference, err := chooseField("reference", overrides.Reference, findReferenceCandidates(text), false)
	if err != nil {
		return SuggestedPaymentDetailsReport{}, err
	}

	details := SuggestedPaymentDetails{
		Payee:     payee,
		IBAN:      iban,
		Amount:    amount,
		Reference: reference,
		BIC:       strings.TrimSpace(overrides.BIC),
	}
	return SuggestedPaymentDetailsReport{
		Payee:        suggestedField(text, "payee", overrides.Payee, payee),
		IBAN:         suggestedField(text, "iban", overrides.IBAN, iban),
		Amount:       suggestedField(text, "amount", overrides.Amount, amount),
		Reference:    suggestedField(text, "reference", overrides.Reference, reference),
		BIC:          suggestedField(text, "bic", overrides.BIC, details.BIC),
		AgentContext: buildAgentContext(text),
		Details:      details,
	}, nil
}

func suggestedPaymentDetails(details SuggestedPaymentDetails) PaymentDetails {
	return PaymentDetails{
		Payee:     details.Payee,
		IBAN:      details.IBAN,
		Amount:    details.Amount,
		Reference: details.Reference,
		BIC:       details.BIC,
	}
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

func suggestedField(text, name, override, value string) SuggestedPaymentField {
	if strings.TrimSpace(override) != "" {
		return SuggestedPaymentField{
			Value:  strings.TrimSpace(override),
			Source: "override",
		}
	}
	if value == "" {
		return SuggestedPaymentField{}
	}
	return SuggestedPaymentField{
		Value:    value,
		Source:   "text",
		Evidence: evidenceLine(text, name, value),
	}
}

func evidenceLine(text, name, value string) string {
	value = strings.TrimSpace(value)
	if name == "amount" {
		return amountEvidenceLine(text, value)
	}
	for _, line := range strings.Split(text, "\n") {
		if lineMatchesEvidence(line, name, value) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func amountEvidenceLine(text, value string) string {
	lines := strings.Split(text, "\n")
	if line := amountEvidenceLineNearLines(lines, amountDueLinePattern, true, value); line != "" {
		return line
	}
	return amountEvidenceLineNearLines(lines, amountLinePattern, false, value)
}

func amountEvidenceLineNearLines(lines []string, pattern *regexp.Regexp, allowNextLine bool, value string) string {
	for index, line := range lines {
		if !pattern.MatchString(line) {
			continue
		}
		if allowNextLine {
			if amountLineMatchesValue(line, findPreferredAmountCandidatesInLine(line), value) {
				return strings.TrimSpace(line)
			}
			candidateLine, candidates := findStandaloneCurrencyAmountCandidateLine(lines, index)
			if amountLineMatchesValue(candidateLine, candidates, value) {
				return strings.TrimSpace(candidateLine)
			}
			continue
		}
		if amountLineMatchesValue(line, findAmountCandidatesInLine(line), value) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func findStandaloneCurrencyAmountCandidateLine(lines []string, index int) (string, []string) {
	for _, line := range lines[index+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		return line, findStandaloneCurrencyAmountCandidatesInLine(line)
	}
	return "", nil
}

func amountLineMatchesValue(line string, candidates []string, value string) bool {
	if strings.TrimSpace(line) == "" {
		return false
	}
	for _, candidate := range candidates {
		normalized, err := normalizeSuggestedAmount(candidate)
		if err == nil && normalized == value {
			return true
		}
	}
	return false
}

func lineMatchesEvidence(line, name, value string) bool {
	if strings.Contains(line, value) {
		return true
	}
	switch name {
	case "iban":
		return strings.Contains(compactAlnumUpper(line), compactAlnumUpper(value))
	case "amount":
		candidates := append(findAmountCandidatesInLine(line), findStandaloneCurrencyAmountCandidatesInLine(line)...)
		for _, candidate := range candidates {
			normalized, err := normalizeSuggestedAmount(candidate)
			if err == nil && normalized == value {
				return true
			}
		}
	}
	return false
}

func compactAlnumUpper(input string) string {
	var builder strings.Builder
	for _, r := range input {
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			builder.WriteRune(r)
		}
	}
	return strings.ToUpper(builder.String())
}

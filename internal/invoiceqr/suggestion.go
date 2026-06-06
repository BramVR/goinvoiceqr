package invoiceqr

import "fmt"

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
	Text            string
	Overrides       PaymentDetails
	Output          QROutputOptions
	IncludeFullText bool
}

type SuggestedPaymentArtifactPlan struct {
	Report    SuggestedPaymentDetailsReport
	HasReport bool
	Plan      PaymentArtifactPlan
	HasPlan   bool
}

type SuggestionFieldIssue struct {
	Field  string
	Reason string
}

type IncompleteSuggestionError struct {
	Issues []SuggestionFieldIssue
}

func (err IncompleteSuggestionError) Error() string {
	issue, ok := err.PrimaryIssue()
	if !ok {
		return "suggestion: incomplete"
	}
	return fmt.Sprintf("%s: %s", issue.Field, issue.Reason)
}

func (err IncompleteSuggestionError) PrimaryIssue() (SuggestionFieldIssue, bool) {
	if len(err.Issues) == 0 {
		return SuggestionFieldIssue{}, false
	}
	return err.Issues[0], true
}

func (err IncompleteSuggestionError) MissingFields() []string {
	return err.fieldsWithReason("required")
}

func (err IncompleteSuggestionError) AmbiguousFields() []string {
	return err.fieldsWithReason("ambiguous")
}

func (err IncompleteSuggestionError) fieldsWithReason(reason string) []string {
	fields := []string{}
	for _, issue := range err.Issues {
		if issue.Reason == reason {
			fields = append(fields, issue.Field)
		}
	}
	return fields
}

func SuggestPaymentDetailsFromText(text string, overrides PaymentDetails) (SuggestedPaymentDetails, error) {
	report, err := SuggestPaymentDetailsReportFromText(text, overrides)
	if err != nil {
		return SuggestedPaymentDetails{}, err
	}
	return report.Details, nil
}

func BuildSuggestedPaymentArtifactPlan(options SuggestedPaymentArtifactPlanOptions) (SuggestedPaymentArtifactPlan, error) {
	report, err := suggestPaymentDetailsReportFromText(options.Text, options.Overrides, options.IncludeFullText)
	if err != nil {
		return SuggestedPaymentArtifactPlan{Report: report, HasReport: true}, err
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
	return suggestPaymentDetailsReportFromText(text, overrides, false)
}

func suggestPaymentDetailsReportFromText(text string, overrides PaymentDetails, includeFullText bool) (SuggestedPaymentDetailsReport, error) {
	selection := selectSuggestionEvidence(suggestionEvidenceFromText(text), overrides)

	report := SuggestedPaymentDetailsReport{
		Payee:        selection.Payee,
		IBAN:         selection.IBAN,
		Amount:       selection.Amount,
		Reference:    selection.Reference,
		BIC:          selection.BIC,
		AgentContext: buildAgentContext(text, includeFullText, selection.Candidates, selection.ReviewCandidates),
		Details:      selection.Details,
	}
	if len(selection.Issues) > 0 {
		return report, IncompleteSuggestionError{Issues: selection.Issues}
	}
	return report, nil
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

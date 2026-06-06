package invoiceqr

import "strings"

const (
	suggestionEvidenceStatusCandidate = "candidate"
	suggestionEvidenceStatusReview    = "review"
)

type suggestionEvidence struct {
	Field        string
	Value        string
	Normalized   string
	Evidence     string
	Line         int
	Kind         string
	Status       string
	ReviewReason string
}

type suggestionSelection struct {
	Payee            SuggestedPaymentField
	IBAN             SuggestedPaymentField
	Amount           SuggestedPaymentField
	Reference        SuggestedPaymentField
	BIC              SuggestedPaymentField
	Details          SuggestedPaymentDetails
	Issues           []SuggestionFieldIssue
	Candidates       AgentContextCandidates
	ReviewCandidates AgentContextCandidates
}

func suggestionEvidenceFromText(text string) []suggestionEvidence {
	evidence := []suggestionEvidence{}
	evidence = append(evidence, suggestionPayeeEvidenceFromText(text)...)
	evidence = appendAgentContextEvidence(evidence, "iban", agentContextIBANCandidates(text), suggestionEvidenceStatusCandidate, "")
	evidence = append(evidence, suggestionAmountEvidenceFromText(text)...)
	evidence = appendAgentContextEvidence(evidence, "reference", agentContextReferenceCandidates(text), suggestionEvidenceStatusCandidate, "")
	return evidence
}

func suggestionPayeeEvidenceFromText(text string) []suggestionEvidence {
	payeeCandidates := agentContextPayeeCandidates(text)
	evidence := appendAgentContextEvidence(nil, "payee", payeeCandidates, suggestionEvidenceStatusCandidate, "")
	if len(payeeCandidates) == 0 {
		evidence = appendAgentContextEvidence(evidence, "payee", footerBrandPayeeReviewCandidates(text), suggestionEvidenceStatusReview, "weak_footer_brand")
	}
	return evidence
}

func suggestionAmountEvidenceFromText(text string) []suggestionEvidence {
	if paymentInstructionCandidates := agentContextPaymentInstructionAmountCandidates(text); len(paymentInstructionCandidates) > 0 {
		evidence := appendAgentContextEvidence(nil, "amount", paymentInstructionCandidates, suggestionEvidenceStatusCandidate, "")
		if selectedAmount, ok := singleNormalizedAmount(paymentInstructionCandidates); ok {
			conflicts := conflictingAmountCandidates(agentContextConflictingGenericAmountCandidates(text), selectedAmount)
			evidence = appendAgentContextEvidence(evidence, "amount", conflicts, suggestionEvidenceStatusReview, "conflicting_generic_total")
		}
		return evidence
	}
	if payableTotalCandidates := agentContextPayableTotalAmountCandidates(text); len(payableTotalCandidates) > 0 {
		return appendAgentContextEvidence(nil, "amount", payableTotalCandidates, suggestionEvidenceStatusCandidate, "")
	}
	return appendAgentContextEvidence(nil, "amount", agentContextGenericAmountCandidates(text), suggestionEvidenceStatusCandidate, "")
}

func singleNormalizedAmount(candidates []AgentContextCandidate) (string, bool) {
	selected := ""
	for _, candidate := range candidates {
		value := candidate.Normalized
		if value == "" {
			value = strings.TrimSpace(candidate.Value)
		}
		if value == "" {
			continue
		}
		if selected == "" {
			selected = value
			continue
		}
		if selected != value {
			return "", false
		}
	}
	return selected, selected != ""
}

func conflictingAmountCandidates(candidates []AgentContextCandidate, selected string) []AgentContextCandidate {
	conflicts := []AgentContextCandidate{}
	for _, candidate := range candidates {
		value := candidate.Normalized
		if value == "" {
			value = strings.TrimSpace(candidate.Value)
		}
		if value != "" && value != selected {
			conflicts = append(conflicts, candidate)
		}
	}
	return conflicts
}

func appendAgentContextEvidence(evidence []suggestionEvidence, field string, candidates []AgentContextCandidate, status, reason string) []suggestionEvidence {
	for _, candidate := range candidates {
		evidence = append(evidence, suggestionEvidence{
			Field:        field,
			Value:        candidate.Value,
			Normalized:   candidate.Normalized,
			Evidence:     candidate.Evidence,
			Line:         candidate.Line,
			Kind:         candidate.Kind,
			Status:       status,
			ReviewReason: reason,
		})
	}
	return evidence
}

func selectSuggestionEvidence(evidence []suggestionEvidence, overrides PaymentDetails) suggestionSelection {
	selection := suggestionSelection{
		Candidates:       agentContextCandidatesFromEvidence(evidence, suggestionEvidenceStatusCandidate),
		ReviewCandidates: agentContextCandidatesFromEvidence(evidence, suggestionEvidenceStatusReview),
	}

	selection.Payee, selection.Details.Payee = selectPayeeField(overrides.Payee, evidence, &selection.Issues)
	selection.IBAN, selection.Details.IBAN = selectRequiredField("iban", overrides.IBAN, evidence, true, &selection.Issues)
	selection.Amount, selection.Details.Amount = selectRequiredField("amount", overrides.Amount, evidence, true, &selection.Issues)
	selection.Reference, selection.Details.Reference = selectRequiredField("reference", overrides.Reference, evidence, false, &selection.Issues)

	if strings.TrimSpace(overrides.BIC) != "" {
		value := strings.TrimSpace(overrides.BIC)
		selection.BIC = SuggestedPaymentField{Value: value, Source: "override"}
		selection.Details.BIC = value
	}

	return selection
}

func selectPayeeField(override string, evidence []suggestionEvidence, issues *[]SuggestionFieldIssue) (SuggestedPaymentField, string) {
	if strings.TrimSpace(override) != "" {
		value := strings.TrimSpace(override)
		return SuggestedPaymentField{Value: value, Source: "override"}, value
	}

	candidates := selectableEvidence("payee", evidence)
	if len(candidates) == 0 {
		*issues = append(*issues, SuggestionFieldIssue{Field: "payee", Reason: "required"})
		return SuggestedPaymentField{}, ""
	}

	if selected, ok := firstPayeeEvidence(candidates, explicitPayeeEvidence); ok {
		value := selectedEvidenceValue(selected)
		return SuggestedPaymentField{Value: value, Source: "text", Evidence: selected.Evidence}, value
	}
	if selected, ok := firstPayeeEvidence(candidates, strongInferredPayeeEvidence); ok {
		value := selectedEvidenceValue(selected)
		return SuggestedPaymentField{Value: value, Source: "text", Evidence: selected.Evidence}, value
	}

	if len(candidates) > 1 {
		*issues = append(*issues, SuggestionFieldIssue{Field: "payee", Reason: "ambiguous"})
		return SuggestedPaymentField{}, ""
	}
	value := selectedEvidenceValue(candidates[0])
	return SuggestedPaymentField{Value: value, Source: "text", Evidence: candidates[0].Evidence}, value
}

func firstPayeeEvidence(candidates []suggestionEvidence, match func(suggestionEvidence) bool) (suggestionEvidence, bool) {
	for _, candidate := range candidates {
		if match(candidate) {
			return candidate, true
		}
	}
	return suggestionEvidence{}, false
}

func explicitPayeeEvidence(evidence suggestionEvidence) bool {
	return payeeLinePattern.MatchString(evidence.Evidence)
}

func strongInferredPayeeEvidence(evidence suggestionEvidence) bool {
	return evidence.Kind != "footer_brand" && !explicitPayeeEvidence(evidence)
}

func selectRequiredField(name, override string, evidence []suggestionEvidence, ambiguous bool, issues *[]SuggestionFieldIssue) (SuggestedPaymentField, string) {
	if strings.TrimSpace(override) != "" {
		value := strings.TrimSpace(override)
		return SuggestedPaymentField{Value: value, Source: "override"}, value
	}

	candidates := selectableEvidence(name, evidence)
	switch len(candidates) {
	case 0:
		*issues = append(*issues, SuggestionFieldIssue{Field: name, Reason: "required"})
		return SuggestedPaymentField{}, ""
	case 1:
		value := selectedEvidenceValue(candidates[0])
		return SuggestedPaymentField{Value: value, Source: "text", Evidence: candidates[0].Evidence}, value
	default:
		if ambiguous {
			*issues = append(*issues, SuggestionFieldIssue{Field: name, Reason: "ambiguous"})
			return SuggestedPaymentField{}, ""
		}
		value := selectedEvidenceValue(candidates[0])
		return SuggestedPaymentField{Value: value, Source: "text", Evidence: candidates[0].Evidence}, value
	}
}

func selectableEvidence(field string, evidence []suggestionEvidence) []suggestionEvidence {
	candidates := []suggestionEvidence{}
	seen := map[string]bool{}
	for _, item := range prioritizedFieldEvidence(field, evidence) {
		if item.Field != field || item.Status != suggestionEvidenceStatusCandidate {
			continue
		}
		value := selectedEvidenceValue(item)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		candidates = append(candidates, item)
	}
	return candidates
}

func prioritizedFieldEvidence(field string, evidence []suggestionEvidence) []suggestionEvidence {
	if field == "payee" {
		prioritized := []suggestionEvidence{}
		for _, item := range evidence {
			if item.Field == field && payeeLinePattern.MatchString(item.Evidence) {
				prioritized = append(prioritized, item)
			}
		}
		for _, item := range evidence {
			if item.Field == field && !payeeLinePattern.MatchString(item.Evidence) {
				prioritized = append(prioritized, item)
			}
		}
		return prioritized
	}
	if field != "reference" {
		return evidence
	}
	prioritized := []suggestionEvidence{}
	for _, item := range evidence {
		if item.Field == field && structuredReferenceEvidence(item) {
			prioritized = append(prioritized, item)
		}
	}
	for _, item := range evidence {
		if item.Field == field && !structuredReferenceEvidence(item) {
			prioritized = append(prioritized, item)
		}
	}
	return prioritized
}

func structuredReferenceEvidence(evidence suggestionEvidence) bool {
	return evidence.Kind == string(StructuredReference) || structuredRefPattern.MatchString(evidence.Value)
}

func selectedEvidenceValue(evidence suggestionEvidence) string {
	if evidence.Field == "amount" && evidence.Normalized != "" {
		return evidence.Normalized
	}
	return strings.TrimSpace(evidence.Value)
}

func agentContextCandidatesFromEvidence(evidence []suggestionEvidence, status string) AgentContextCandidates {
	candidates := AgentContextCandidates{}
	for _, item := range evidence {
		if item.Status != status {
			continue
		}
		candidate := AgentContextCandidate{
			Value:      item.Value,
			Normalized: item.Normalized,
			Evidence:   item.Evidence,
			Line:       item.Line,
			Kind:       item.Kind,
			Reason:     item.ReviewReason,
		}
		switch item.Field {
		case "payee":
			candidates.Payee = appendUniqueAgentCandidate(candidates.Payee, candidate)
		case "iban":
			candidates.IBAN = appendUniqueAgentCandidate(candidates.IBAN, candidate)
		case "amount":
			candidates.Amount = appendUniqueAgentCandidate(candidates.Amount, candidate)
		case "reference":
			candidates.Reference = appendUniqueAgentCandidate(candidates.Reference, candidate)
		}
	}
	return candidates
}

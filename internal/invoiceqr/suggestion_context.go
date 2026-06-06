package invoiceqr

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

var paymentInstructionPattern = regexp.MustCompile(`(?i)\b(?:pay|payments?|payable|betaal\w*|betal\w*)\b`)

const (
	amountCandidateKindPaymentInstruction = "payment_instruction"
	amountCandidateKindPayableTotal       = "payable_total"
	amountCandidateKindGenericTotal       = "generic_total"
)

type AgentContext struct {
	SourceTextHash   string
	FullText         string
	ObservedLines    []AgentContextObservedLine
	Candidates       AgentContextCandidates
	ReviewCandidates AgentContextCandidates
}

type AgentContextObservedLine struct {
	Kind string
	Line int
	Text string
}

type AgentContextCandidates struct {
	Payee     []AgentContextCandidate
	IBAN      []AgentContextCandidate
	Amount    []AgentContextCandidate
	Reference []AgentContextCandidate
}

type AgentContextCandidate struct {
	Value      string
	Normalized string
	Evidence   string
	Line       int
	Kind       string
	Reason     string
}

func buildAgentContext(text string, includeFullText bool, candidates, reviewCandidates AgentContextCandidates) AgentContext {
	hash := sha256.Sum256([]byte(text))
	context := AgentContext{
		SourceTextHash:   fmt.Sprintf("sha256:%x", hash),
		ObservedLines:    agentContextObservedLines(text),
		Candidates:       candidates,
		ReviewCandidates: reviewCandidates,
	}
	if includeFullText {
		context.FullText = text
	}
	return context
}

func agentContextObservedLines(text string) []AgentContextObservedLine {
	lines := strings.Split(text, "\n")
	observed := []AgentContextObservedLine{}
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		kind := agentContextLineKind(lines, index)
		if kind == "" {
			continue
		}
		observed = appendUniqueObservedLine(observed, AgentContextObservedLine{
			Kind: kind,
			Line: index + 1,
			Text: trimmed,
		})
	}
	return observed
}

func agentContextLineKind(lines []string, index int) string {
	line := strings.TrimSpace(lines[index])
	switch {
	case index == firstNonEmptyLineIndex(lines) && strings.Contains(strings.ToLower(line), "invoice"):
		return "document_header"
	case payeeLinePattern.MatchString(line) || legalEntityNamePattern.MatchString(line):
		return "payee_context"
	case strings.Contains(strings.ToLower(line), "iban"):
		return "iban_context"
	case structuredRefPattern.MatchString(line) || referenceLinePattern.MatchString(line):
		return "reference_context"
	case amountDueLinePattern.MatchString(line) || amountLinePattern.MatchString(line) || len(findStandaloneCurrencyAmountCandidatesInLine(line)) > 0:
		return "amount_context"
	case paymentInstructionLine(line):
		return "payment_instruction"
	case index == lastNonEmptyLineIndex(lines):
		return "document_footer"
	default:
		return ""
	}
}

func paymentInstructionLine(line string) bool {
	return paymentInstructionPattern.MatchString(line)
}

func firstNonEmptyLineIndex(lines []string) int {
	for index, line := range lines {
		if strings.TrimSpace(line) != "" {
			return index
		}
	}
	return -1
}

func lastNonEmptyLineIndex(lines []string) int {
	for index := len(lines) - 1; index >= 0; index-- {
		if strings.TrimSpace(lines[index]) != "" {
			return index
		}
	}
	return -1
}

func appendUniqueObservedLine(lines []AgentContextObservedLine, line AgentContextObservedLine) []AgentContextObservedLine {
	for _, existing := range lines {
		if existing.Kind == line.Kind && existing.Line == line.Line && existing.Text == line.Text {
			return lines
		}
	}
	return append(lines, line)
}

func agentContextPayeeCandidates(text string) []AgentContextCandidate {
	candidates := []AgentContextCandidate{}
	for index, line := range strings.Split(text, "\n") {
		if match := payeeLinePattern.FindStringSubmatch(line); len(match) > 1 {
			value := strings.TrimSpace(match[1])
			if creditor, ok := findCreditorIBANLinePayee(line); ok {
				value = creditor
			}
			candidates = appendUniqueAgentCandidate(candidates, AgentContextCandidate{
				Value:    value,
				Evidence: strings.TrimSpace(line),
				Line:     index + 1,
			})
			continue
		}
		if creditor, ok := findCreditorIBANLinePayee(line); ok {
			candidates = appendUniqueAgentCandidate(candidates, AgentContextCandidate{
				Value:    creditor,
				Evidence: strings.TrimSpace(line),
				Line:     index + 1,
			})
		}
	}
	return candidates
}

func agentContextIBANCandidates(text string) []AgentContextCandidate {
	candidates := []AgentContextCandidate{}
	seen := map[string]bool{}
	for index, line := range strings.Split(text, "\n") {
		for _, match := range ibanCandidatePattern.FindAllString(line, -1) {
			normalized, err := normalizeIBAN(match)
			if err != nil || seen[normalized] {
				continue
			}
			seen[normalized] = true
			candidates = append(candidates, AgentContextCandidate{
				Value:      strings.TrimSpace(match),
				Normalized: normalized,
				Evidence:   strings.TrimSpace(line),
				Line:       index + 1,
			})
		}
	}
	return candidates
}

func agentContextPaymentInstructionAmountCandidates(text string) []AgentContextCandidate {
	lines := strings.Split(text, "\n")
	candidates := []AgentContextCandidate{}
	seen := map[string]bool{}
	for index, line := range lines {
		if amountDueLinePattern.MatchString(line) {
			continue
		}
		for _, value := range findPaymentInstructionAmountCandidatesInLine(line) {
			candidates = appendAgentContextAmountCandidate(candidates, seen, value, line, index+1, amountCandidateKindPaymentInstruction)
		}
	}
	return candidates
}

func agentContextPayableTotalAmountCandidates(text string) []AgentContextCandidate {
	return agentContextAmountCandidatesNearLines(text, amountDueLinePattern, true, amountCandidateKindPayableTotal)
}

func agentContextGenericAmountCandidates(text string) []AgentContextCandidate {
	return agentContextGenericAmountCandidatesWithPayable(text, true)
}

func agentContextConflictingGenericAmountCandidates(text string) []AgentContextCandidate {
	return agentContextGenericAmountCandidatesWithPayable(text, false)
}

func agentContextGenericAmountCandidatesWithPayable(text string, includePayable bool) []AgentContextCandidate {
	lines := strings.Split(text, "\n")
	candidates := []AgentContextCandidate{}
	seen := map[string]bool{}
	for index, line := range lines {
		if !includePayable && amountDueLinePattern.MatchString(line) {
			continue
		}
		if !amountLinePattern.MatchString(line) {
			continue
		}
		for _, value := range findAmountCandidatesInLine(line) {
			candidates = appendAgentContextAmountCandidate(candidates, seen, value, line, index+1, amountCandidateKindGenericTotal)
		}
	}
	return candidates
}

func agentContextAmountCandidatesNearLines(text string, pattern *regexp.Regexp, allowNextLine bool, kind string) []AgentContextCandidate {
	lines := strings.Split(text, "\n")
	candidates := []AgentContextCandidate{}
	seen := map[string]bool{}
	for index, line := range lines {
		if !pattern.MatchString(line) {
			continue
		}
		candidateLineIndex := index
		values := findAmountCandidatesInLine(line)
		if allowNextLine {
			values = findPreferredAmountCandidatesInLine(line)
			if len(values) == 0 {
				candidateLineIndex, values = agentContextNextStandaloneAmountCandidates(lines, index)
			}
		}
		for _, value := range values {
			candidates = appendAgentContextAmountCandidate(candidates, seen, value, lines[candidateLineIndex], candidateLineIndex+1, kind)
		}
	}
	return candidates
}

func appendAgentContextAmountCandidate(candidates []AgentContextCandidate, seen map[string]bool, value, evidence string, line int, kind string) []AgentContextCandidate {
	normalized, err := normalizeSuggestedAmount(value)
	if err != nil || seen[normalized] {
		return candidates
	}
	seen[normalized] = true
	return append(candidates, AgentContextCandidate{
		Value:      strings.TrimSpace(value),
		Normalized: normalized,
		Evidence:   strings.TrimSpace(evidence),
		Line:       line,
		Kind:       kind,
	})
}

func agentContextNextStandaloneAmountCandidates(lines []string, index int) (int, []string) {
	for nextIndex := index + 1; nextIndex < len(lines); nextIndex++ {
		if strings.TrimSpace(lines[nextIndex]) == "" {
			continue
		}
		return nextIndex, findStandaloneCurrencyAmountCandidatesInLine(lines[nextIndex])
	}
	return index, nil
}

func agentContextReferenceCandidates(text string) []AgentContextCandidate {
	candidates := []AgentContextCandidate{}
	for index, line := range strings.Split(text, "\n") {
		for _, match := range structuredRefPattern.FindAllString(line, -1) {
			candidate := AgentContextCandidate{
				Value:    strings.TrimSpace(match),
				Evidence: strings.TrimSpace(line),
				Line:     index + 1,
			}
			if reference, err := classifyRemittanceReference(match); err == nil {
				candidate.Normalized = reference.Value
				candidate.Kind = string(reference.Kind)
			}
			candidates = appendUniqueAgentCandidate(candidates, candidate)
		}
		if structuredRefPattern.MatchString(line) {
			continue
		}
		if match := referenceLinePattern.FindStringSubmatch(line); len(match) > 1 {
			value := strings.TrimSpace(match[1])
			candidate := AgentContextCandidate{
				Value:    value,
				Evidence: strings.TrimSpace(line),
				Line:     index + 1,
			}
			if reference, err := classifyRemittanceReference(value); err == nil {
				candidate.Normalized = reference.Value
				candidate.Kind = string(reference.Kind)
			}
			candidates = appendUniqueAgentCandidate(candidates, candidate)
		}
	}
	return candidates
}

func appendUniqueAgentCandidate(candidates []AgentContextCandidate, candidate AgentContextCandidate) []AgentContextCandidate {
	if candidate.Value == "" {
		return candidates
	}
	for _, existing := range candidates {
		if existing.Value == candidate.Value && existing.Normalized == candidate.Normalized && existing.Kind == candidate.Kind {
			return candidates
		}
	}
	return append(candidates, candidate)
}

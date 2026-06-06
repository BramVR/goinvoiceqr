package invoiceqr

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	payeeLinePattern             = regexp.MustCompile(`(?im)^\s*(?:payee|beneficiary|supplier|begunstigde|leverancier)\s*:\s*(.+?)\s*$`)
	creditorIBANLineLabelPattern = regexp.MustCompile(`(?i)^\s*(?:creditor|payee|beneficiary|supplier|begunstigde|leverancier)\s*:\s*`)
	creditorIBANMarkerPattern    = regexp.MustCompile(`(?i)\bIBAN\b\s*:?\s*[A-Z]{2}[ \t]*[0-9]{2}(?:[ \t]*[A-Z0-9]){10,30}\b`)
	customerDetailLinePattern    = regexp.MustCompile(`(?i)^\s*(?:name|customer(?:\s+name)?|client(?:\s+name)?|billing(?:\s+name)?|delivery(?:\s+name)?|ship(?:ped)?\s*to|bill(?:ed)?\s*to|invoice\s*to|klant|naam)\s*:`)
	legalEntitySuffixPattern     = regexp.MustCompile(`(?i)(?:^|\s)(?:B\.V\.|BVBA|BV|N\.V\.|NV|CV|GmbH|SARL|S\.A\.|SA|Ltd|Limited|Inc|LLC|VZW|ASBL)$`)
	legalEntityNamePattern       = regexp.MustCompile(`(?i)^(.+?\b(?:B\.V\.|BVBA|BV|N\.V\.|NV|CV|GmbH|SARL|S\.A\.|SA|Ltd|Limited|Inc|LLC|VZW|ASBL))(?:\s|$)`)
	footerBrandAdjacentPattern   = regexp.MustCompile(`(?i)\b(?:IBAN|BIC|VAT|BTW|TVA|BE\s*\d{4}[.\s]?\d{3}[.\s]?\d{3})\b|@|https?://|www\.|\b(?:tel|phone|email|mail)\b`)
)

func findCreditorIBANLinePayee(line string) (string, bool) {
	ibanLocation := creditorIBANMarkerPattern.FindStringIndex(line)
	if ibanLocation == nil {
		return "", false
	}
	prefix := creditorIBANLineLabelPattern.ReplaceAllString(line[:ibanLocation[0]], "")
	for _, segment := range creditorIBANLineSegments(prefix) {
		if candidate, ok := legalEntityNameCandidate(segment); ok {
			return candidate, true
		}
	}
	return "", false
}

func legalEntityNameCandidate(segment string) (string, bool) {
	match := legalEntityNamePattern.FindStringSubmatch(strings.TrimSpace(segment))
	if len(match) < 2 {
		return "", false
	}
	return strings.TrimSpace(match[1]), true
}

func creditorIBANLineSegments(prefix string) []string {
	segments := []string{}
	var segment strings.Builder
	runes := []rune(prefix)
	for index, r := range runes {
		if !creditorIBANDash(r) {
			segment.WriteRune(r)
			continue
		}
		previousSpace := index == 0 || unicode.IsSpace(runes[index-1])
		nextSpace := index == len(runes)-1 || unicode.IsSpace(runes[index+1])
		if previousSpace || nextSpace || legalEntitySuffixPattern.MatchString(strings.TrimSpace(segment.String())) {
			segments = append(segments, segment.String())
			segment.Reset()
			continue
		}
		segment.WriteRune(r)
	}
	segments = append(segments, segment.String())
	return segments
}

func creditorIBANDash(r rune) bool {
	return r == '-' || r == '–' || r == '—'
}

func footerBrandPayeeCandidates(text string) []AgentContextCandidate {
	selectable, _ := footerBrandPayeeCandidatesByStatus(text)
	return selectable
}

func footerBrandPayeeReviewCandidates(text string) []AgentContextCandidate {
	_, review := footerBrandPayeeCandidatesByStatus(text)
	return review
}

func footerBrandPayeeCandidatesByStatus(text string) ([]AgentContextCandidate, []AgentContextCandidate) {
	lines := strings.Split(text, "\n")
	footerStartLine, ok := footerBrandContextStartLine(lines)
	if !ok {
		return nil, nil
	}
	occurrences := map[string][]AgentContextCandidate{}
	order := []string{}
	for index, line := range lines {
		lineNumber := index + 1
		if lineNumber < footerStartLine {
			continue
		}
		if payeeLinePattern.MatchString(line) || customerDetailLinePattern.MatchString(line) {
			continue
		}
		candidate, ok := legalEntityNameCandidate(line)
		if !ok {
			continue
		}
		if len(occurrences[candidate]) == 0 {
			order = append(order, candidate)
		}
		occurrences[candidate] = append(occurrences[candidate], AgentContextCandidate{
			Value:    candidate,
			Evidence: strings.TrimSpace(line),
			Line:     lineNumber,
		})
	}

	candidates := []AgentContextCandidate{}
	reviewCandidates := []AgentContextCandidate{}
	for _, value := range order {
		matches := occurrences[value]
		if !footerBrandBankAdjacent(lines, matches) {
			continue
		}
		candidate := AgentContextCandidate{
			Value:    value,
			Evidence: matches[0].Evidence,
			Line:     matches[0].Line,
			Kind:     "footer_brand",
		}
		if len(matches) < 2 {
			candidate.Reason = "weak_footer_brand"
			reviewCandidates = appendUniqueAgentCandidate(reviewCandidates, candidate)
			continue
		}
		candidates = appendUniqueAgentCandidate(candidates, candidate)
	}
	return candidates, reviewCandidates
}

func footerBrandContextStartLine(lines []string) (int, bool) {
	for index, line := range lines {
		if footerBrandContextBoundaryLine(line) {
			return index + 2, true
		}
	}
	return 0, false
}

func footerBrandContextBoundaryLine(line string) bool {
	if amountDueLinePattern.MatchString(line) || amountLinePattern.MatchString(line) || structuredRefPattern.MatchString(line) {
		return true
	}
	trimmed := strings.ToLower(strings.TrimSpace(line))
	return !strings.HasPrefix(trimmed, "invoice") && referenceLinePattern.MatchString(line)
}

func footerBrandBankAdjacent(lines []string, matches []AgentContextCandidate) bool {
	for _, match := range matches {
		index := match.Line - 1
		for adjacent := index - 1; adjacent <= index+1; adjacent++ {
			if adjacent < 0 || adjacent >= len(lines) || adjacent == index {
				continue
			}
			if footerBrandAdjacentPattern.MatchString(lines[adjacent]) {
				return true
			}
		}
	}
	return false
}

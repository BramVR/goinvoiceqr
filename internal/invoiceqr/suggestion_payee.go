package invoiceqr

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	payeeLinePattern             = regexp.MustCompile(`(?im)^\s*(?:payee|beneficiary|supplier|name|begunstigde|leverancier)\s*:\s*(.+?)\s*$`)
	creditorIBANLineLabelPattern = regexp.MustCompile(`(?i)^\s*(?:creditor|payee|beneficiary|supplier|begunstigde|leverancier)\s*:\s*`)
	creditorIBANMarkerPattern    = regexp.MustCompile(`(?i)\bIBAN\b\s*:?\s*[A-Z]{2}[ \t]*[0-9]{2}(?:[ \t]*[A-Z0-9]){10,30}\b`)
	legalEntitySuffixPattern     = regexp.MustCompile(`(?i)(?:^|\s)(?:B\.V\.|BVBA|BV|N\.V\.|NV|GmbH|SARL|S\.A\.|SA|Ltd|Limited|Inc|LLC|VZW|ASBL)$`)
	legalEntityNamePattern       = regexp.MustCompile(`(?i)^(.+?\b(?:B\.V\.|BVBA|BV|N\.V\.|NV|GmbH|SARL|S\.A\.|SA|Ltd|Limited|Inc|LLC|VZW|ASBL))(?:\s|$)`)
)

func findPayeeCandidates(text string) []string {
	explicit := []string{}
	inferred := []string{}
	for _, line := range strings.Split(text, "\n") {
		if match := payeeLinePattern.FindStringSubmatch(line); len(match) > 1 {
			if creditor, ok := findCreditorIBANLinePayee(line); ok {
				explicit = appendUnique(explicit, creditor)
				continue
			}
			explicit = appendUnique(explicit, strings.TrimSpace(match[1]))
			continue
		}
		if creditor, ok := findCreditorIBANLinePayee(line); ok {
			inferred = appendUnique(inferred, creditor)
		}
	}
	return append(explicit, inferred...)
}

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

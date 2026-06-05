package invoiceqr

import (
	"regexp"
	"strings"
)

var (
	referenceLinePattern = regexp.MustCompile(`(?im)^\s*(?:reference|communication|remittance|mededeling|invoice)\s*:\s*(.+?)\s*$`)
	structuredRefPattern = regexp.MustCompile(`\+\+\+/?\d{3}/\d{4}/\d{5}\+\+\+`)
)

func findReferenceCandidates(text string) []string {
	if match := structuredRefPattern.FindString(text); match != "" {
		return []string{match}
	}
	references := findReferenceLabelCandidates(text)
	if len(references) == 0 {
		return references
	}
	clean := make([]string, 0, len(references))
	for _, reference := range references {
		clean = appendUnique(clean, reference)
	}
	return clean
}

func findReferenceLabelCandidates(text string) []string {
	matches := referenceLinePattern.FindAllStringSubmatch(text, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = appendUnique(values, strings.TrimSpace(match[1]))
	}
	return values
}

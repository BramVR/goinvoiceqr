package invoiceqr

import (
	"regexp"
	"strings"
)

var ibanCandidatePattern = regexp.MustCompile(`(?i)\b[A-Z]{2}[ \t]*[0-9]{2}(?:[ \t]*[A-Z0-9]){10,30}\b`)

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

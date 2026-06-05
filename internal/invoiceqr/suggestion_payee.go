package invoiceqr

import (
	"regexp"
	"strings"
)

var (
	payeeLinePattern        = regexp.MustCompile(`(?im)^\s*(?:payee|beneficiary|supplier|name|begunstigde|leverancier)\s*:\s*(.+?)\s*$`)
	creditorIBANLinePattern = regexp.MustCompile(`(?im)^\s*(?:(?:creditor|payee|beneficiary|supplier|begunstigde|leverancier)\s*:\s*)?([^-–—\n]+?\b(?:BV|B\.V\.|NV|N\.V\.|BVBA|GmbH|SARL|S\.A\.|SA|Ltd|Limited|Inc|LLC|VZW|ASBL))(?:\s|[-–—]|$)[^\n]*\bIBAN\b`)
)

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

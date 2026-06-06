package invoiceqr

import (
	"regexp"
)

var ibanCandidatePattern = regexp.MustCompile(`(?i)\b[A-Z]{2}[ \t]*[0-9]{2}(?:[ \t]*[A-Z0-9]){10,30}\b`)

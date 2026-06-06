package invoiceqr

import (
	"regexp"
)

var (
	referenceLinePattern = regexp.MustCompile(`(?im)^\s*(?:reference|communication|remittance|mededeling|invoice)\s*:\s*(.+?)\s*$`)
	structuredRefPattern = regexp.MustCompile(`\+\+\+/?\d{3}/\d{4}/\d{5}\+\+\+`)
)

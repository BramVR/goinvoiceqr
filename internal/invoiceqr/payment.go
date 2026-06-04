package invoiceqr

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"unicode"
)

const (
	StructuredReference   RemittanceKind = "structured"
	UnstructuredReference RemittanceKind = "unstructured"
)

type RemittanceKind string

type PaymentDetails struct {
	Payee     string
	IBAN      string
	Amount    string
	Reference string
	BIC       string
}

type RemittanceReference struct {
	Kind  RemittanceKind
	Value string
}

type ConfirmedPaymentDetails struct {
	Payee     string
	IBAN      string
	Amount    string
	Reference RemittanceReference
	BIC       string
}

var structuredReferencePattern = regexp.MustCompile(`^\+\+\+/(\d{3})/(\d{4})/(\d{5})\+\+\+$`)

func ValidatePaymentDetails(details PaymentDetails) (ConfirmedPaymentDetails, error) {
	payee := strings.TrimSpace(details.Payee)
	if payee == "" {
		return ConfirmedPaymentDetails{}, errors.New("payee: required")
	}
	if len([]rune(payee)) > 70 {
		return ConfirmedPaymentDetails{}, errors.New("payee: must be at most 70 characters")
	}

	iban, err := normalizeIBAN(details.IBAN)
	if err != nil {
		return ConfirmedPaymentDetails{}, fmt.Errorf("iban: %w", err)
	}

	amount, err := normalizeAmount(details.Amount)
	if err != nil {
		return ConfirmedPaymentDetails{}, fmt.Errorf("amount: %w", err)
	}

	reference, err := classifyRemittanceReference(details.Reference)
	if err != nil {
		return ConfirmedPaymentDetails{}, fmt.Errorf("reference: %w", err)
	}

	bic, err := normalizeBIC(details.BIC)
	if err != nil {
		return ConfirmedPaymentDetails{}, fmt.Errorf("bic: %w", err)
	}

	return ConfirmedPaymentDetails{
		Payee:     payee,
		IBAN:      iban,
		Amount:    amount,
		Reference: reference,
		BIC:       bic,
	}, nil
}

func normalizeIBAN(input string) (string, error) {
	iban := strings.ToUpper(removeSpace(input))
	if len(iban) < 4 {
		return "", errors.New("too short")
	}
	for _, r := range iban {
		if !unicode.IsDigit(r) && (r < 'A' || r > 'Z') {
			return "", errors.New("contains invalid characters")
		}
	}
	length := ibanLength(iban[:2])
	if length == 0 {
		return "", errors.New("unsupported country")
	}
	if !sepaCountry(iban[:2]) {
		return "", errors.New("country is outside SEPA")
	}
	if len(iban) != length {
		return "", errors.New("invalid length")
	}

	rearranged := iban[4:] + iban[:4]
	var numeric strings.Builder
	for _, r := range rearranged {
		switch {
		case unicode.IsDigit(r):
			numeric.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			numeric.WriteString(fmt.Sprintf("%d", int(r-'A')+10))
		default:
			return "", errors.New("contains invalid characters")
		}
	}
	value, ok := new(big.Int).SetString(numeric.String(), 10)
	if !ok {
		return "", errors.New("invalid checksum")
	}
	if new(big.Int).Mod(value, big.NewInt(97)).Int64() != 1 {
		return "", errors.New("invalid checksum")
	}
	return iban, nil
}

func ibanLength(country string) int {
	lengths := map[string]int{
		"AD": 24, "AE": 23, "AL": 28, "AT": 20, "AZ": 28,
		"BA": 20, "BE": 16, "BG": 22, "BH": 22, "BR": 29,
		"CH": 21, "CR": 22, "CY": 28, "CZ": 24,
		"DE": 22, "DK": 18, "DO": 28, "EE": 20, "EG": 29,
		"ES": 24, "FI": 18, "FO": 18, "FR": 27, "GB": 22,
		"GE": 22, "GI": 23, "GL": 18, "GR": 27, "GT": 28,
		"HR": 21, "HU": 28, "IE": 22, "IL": 23, "IS": 26,
		"IT": 27, "JO": 30, "KW": 30, "KZ": 20, "LB": 28,
		"LC": 32, "LI": 21, "LT": 20, "LU": 20, "LV": 21,
		"MC": 27, "MD": 24, "ME": 22, "MK": 19, "MR": 27,
		"MT": 31, "MU": 30, "NL": 18, "NO": 15, "PK": 24,
		"PL": 28, "PS": 29, "PT": 25, "QA": 29, "RO": 24,
		"RS": 22, "SA": 24, "SE": 24, "SI": 19, "SK": 24,
		"SM": 27, "TN": 24, "TR": 26, "UA": 29, "VA": 22,
		"VG": 24, "XK": 20,
	}
	return lengths[country]
}

func sepaCountry(country string) bool {
	countries := map[string]bool{
		"AD": true, "AL": true, "AT": true, "BE": true, "BG": true,
		"CH": true, "CY": true, "CZ": true, "DE": true, "DK": true,
		"EE": true, "ES": true, "FI": true, "FR": true, "GB": true,
		"GI": true, "GR": true, "HR": true,
		"HU": true, "IE": true, "IS": true, "IT": true, "LI": true,
		"LT": true, "LU": true, "LV": true, "MC": true, "MD": true,
		"ME": true, "MK": true, "MT": true, "NL": true, "NO": true,
		"PL": true, "PT": true, "RO": true, "RS": true, "SE": true,
		"SI": true, "SK": true, "SM": true, "VA": true, "XK": true,
	}
	return countries[country]
}

func normalizeAmount(input string) (string, error) {
	amount := strings.TrimSpace(strings.ReplaceAll(input, ",", "."))
	if amount == "" {
		return "", errors.New("required")
	}
	if strings.HasPrefix(amount, "-") {
		return "", errors.New("must be greater than zero")
	}
	parts := strings.Split(amount, ".")
	if len(parts) > 2 || parts[0] == "" {
		return "", errors.New("malformed")
	}
	if len(parts) == 2 && len(parts[1]) > 2 {
		return "", errors.New("must have at most two decimal places")
	}
	for _, part := range parts {
		if part == "" {
			continue
		}
		for _, r := range part {
			if !asciiDigit(r) {
				return "", errors.New("malformed")
			}
		}
	}
	cents := parts[0]
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	if allZero(cents) && allZero(fraction) {
		return "", errors.New("must be greater than zero")
	}
	cents = strings.TrimLeft(cents, "0")
	if cents == "" {
		cents = "0"
	}
	return cents + "." + fraction, nil
}

func asciiDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func classifyRemittanceReference(input string) (RemittanceReference, error) {
	reference := strings.TrimSpace(input)
	if reference == "" {
		return RemittanceReference{}, errors.New("required")
	}
	if len([]rune(reference)) > 140 {
		return RemittanceReference{}, errors.New("must be at most 140 characters")
	}
	if strings.HasPrefix(reference, "+++") || strings.HasSuffix(reference, "+++") {
		match := structuredReferencePattern.FindStringSubmatch(reference)
		if match == nil {
			return RemittanceReference{}, errors.New("malformed Belgian Structured Reference")
		}
		digits := match[1] + match[2] + match[3]
		base := digits[:10]
		check := digits[10:]
		baseValue, _ := new(big.Int).SetString(base, 10)
		remainder := new(big.Int).Mod(baseValue, big.NewInt(97)).Int64()
		if remainder == 0 {
			remainder = 97
		}
		if fmt.Sprintf("%02d", remainder) != check {
			return RemittanceReference{}, errors.New("invalid Belgian Structured Reference checksum")
		}
		return RemittanceReference{Kind: StructuredReference, Value: reference}, nil
	}
	return RemittanceReference{Kind: UnstructuredReference, Value: reference}, nil
}

func normalizeBIC(input string) (string, error) {
	bic := strings.ToUpper(strings.TrimSpace(input))
	if bic == "" {
		return "", nil
	}
	if len(bic) != 8 && len(bic) != 11 {
		return "", errors.New("must be 8 or 11 characters")
	}
	for i, r := range bic {
		switch {
		case i < 4 && r >= 'A' && r <= 'Z':
		case i >= 4 && i < 6 && r >= 'A' && r <= 'Z':
		case i >= 6 && ((r >= 'A' && r <= 'Z') || unicode.IsDigit(r)):
		default:
			return "", errors.New("must be a valid SWIFT/BIC code")
		}
	}
	return bic, nil
}

func removeSpace(input string) string {
	var out strings.Builder
	for _, r := range strings.TrimSpace(input) {
		if !unicode.IsSpace(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func allZero(input string) bool {
	for _, r := range input {
		if r != '0' {
			return false
		}
	}
	return true
}

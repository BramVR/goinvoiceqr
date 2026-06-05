---
summary: "How invoice text becomes conservative Suggested Payment Details before validation, confirmation, and QR output."
read_when: "Changing from-text, from-pdf, suggestion parsing, ambiguity handling, overrides, amount parsing, or confirmation policy."
---

# Suggestions

## Summary

`from-text` reads copied invoice text from a file or stdin, while `from-pdf` extracts text with local `pdftotext`. Both commands ask the Suggestion module for Suggested Payment Details. The module extracts conservative payee, IBAN, amount, and remittance-reference candidates, applies explicit CLI flag overrides, then sends the result through deterministic validation and mandatory confirmation before QR output.

## Read when

Read when changing `from-text`, `from-pdf`, suggestion parsing, amount parsing, IBAN/reference ambiguity handling, explicit flag overrides, stdin/file input, or confirmation policy for suggested details.

## Flow

- CLI adapters read text from a file, stdin, or `pdftotext` PDF extraction.
- The Suggestion module extracts candidates and reports required-field errors for missing details.
- Explicit flags override suggested fields.
- Multiple plausible IBANs or amounts are ambiguity errors unless flags resolve them.
- Suggested Payment Details are validated by the trusted payment-generation path.
- Suggested details always require confirmation before writing a QR artifact.

## Safety

PDF extraction, copied text, OCR, and AI may produce Suggested Payment Details only. They must not create Confirmed Payment Details, skip validation, skip confirmation, or write QR output without the same deterministic payment-generation path used for manual details.

## Amounts

Suggested amounts are parsed as complete numeric tokens. Plain decimals such as `42.50` and `42,50` are accepted, and common thousands formats such as `1.234,56`, `1,234.56`, and `1 234,56` normalize to `1234.56`. Regular, non-breaking, and narrow non-breaking spaces are accepted as grouping whitespace.

When PDF text extraction splits table cells across lines, a preferred label such as `Total amount to pay` may use the next non-empty currency-bearing line as its amount candidate. Preferred amount labels are used before generic `Total` lines so detail-table totals do not make the payable amount ambiguous.

Malformed grouped values are ignored as amount candidates rather than truncated. This prevents an invoice amount such as `1.234,567` from becoming `1.23`.

## Payees

Suggested payees prefer explicit labels such as `Payee:` or `Supplier:`. When an invoice footer combines the creditor name and `IBAN` on one line, the legal-entity name before the address and IBAN may be used as a conservative payee candidate. This creditor-IBAN inference requires an `IBAN` marker followed by an IBAN-shaped value, strips leading creditor/payee labels, and scans dash-separated segments so footer prefixes are ignored without breaking hyphenated legal names.

## Stdin Confirmation

When `from-text` reads invoice text from stdin, confirmation reads from the terminal instead of the already-consumed invoice stream. This keeps piped invoice text outside the trusted path while still requiring an explicit interactive confirmation.

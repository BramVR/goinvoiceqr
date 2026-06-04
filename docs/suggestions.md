---
summary: "How invoice text becomes conservative Suggested Payment Details before validation, confirmation, and QR output."
read_when: "Changing from-text, from-pdf, suggestion parsing, ambiguity handling, overrides, amount parsing, or confirmation policy."
---

# Suggestions

## Summary

`from-text` reads copied invoice text from a file or stdin and asks the Suggestion module for Suggested Payment Details. The module extracts conservative payee, IBAN, amount, and remittance-reference candidates, applies explicit CLI flag overrides, then sends the result through deterministic validation and mandatory confirmation before QR output.

## Read when

Read when changing `from-text`, `from-pdf`, suggestion parsing, amount parsing, IBAN/reference ambiguity handling, explicit flag overrides, stdin/file input, or confirmation policy for suggested details.

## Flow

- CLI adapters read text from a file, stdin, or future PDF extraction.
- The Suggestion module extracts candidates and refuses missing required fields.
- Explicit flags override suggested fields.
- Multiple plausible IBANs or amounts are ambiguity errors unless flags resolve them.
- Suggested Payment Details are validated by the trusted payment-generation path.
- Suggested details always require confirmation before writing a QR artifact.

## Amounts

Suggested amounts are parsed as complete numeric tokens. Plain decimals such as `42.50` and `42,50` are accepted, and common thousands formats such as `1.234,56`, `1,234.56`, and `1 234,56` normalize to `1234.56`. Regular, non-breaking, and narrow non-breaking spaces are accepted as grouping whitespace.

Malformed grouped values are ignored as amount candidates rather than truncated. This prevents an invoice amount such as `1.234,567` from becoming `1.23`.

## Stdin Confirmation

When `from-text` reads invoice text from stdin, confirmation reads from the terminal instead of the already-consumed invoice stream. This keeps piped invoice text outside the trusted path while still requiring an explicit interactive confirmation.

# PRD: invoiceqr

## Problem Statement

Belgian invoice payments often require copying payee details, IBANs, amounts, and remittance references into a banking app by hand. That is slow and error-prone, especially for Belgian structured references where a small typo can misdirect reconciliation.

Users need a local command-line tool that turns payment details into Belgian-compatible SEPA/EPC payment QR codes, while keeping the trusted payment-generation path deterministic, validated, and explicitly confirmed.

## Solution

Build `invoiceqr`, a Go CLI that validates payment details, builds deterministic EPC/SEPA Credit Transfer payloads, and renders QR codes as PNG or SVG.

The tool supports manual input, validation-only checks, copied invoice text, and PDF text extraction. Text and PDF extraction only produce Suggested Payment Details. QR generation only uses Confirmed Payment Details after deterministic validation and explicit confirmation, except that manual `generate --yes` may skip the prompt for scripted use.

## User Stories

1. As a user, I want to generate a QR code from a payee name, IBAN, amount, and reference, so that I can pay an invoice from a banking app.

2. As a user, I want Belgian structured references validated before QR generation, so that typoed payment communications fail early.

3. As a user, I want normal invoice references accepted as unstructured remittance information, so that invoices without Belgian structured references still work.

4. As a user, I want invalid Belgian structured-reference-looking values rejected, so that malformed structured references are not silently treated as free text.

5. As a user, I want IBAN and amount validation, so that generated payment QR codes contain plausible SEPA payment details.

6. As a user, I want the tool to show Payee, IBAN, Amount, and Reference before generation, so that I can confirm the exact payment before a QR is written.

7. As a script author, I want `generate --yes`, so that deterministic manual payment details can be used in automation after validation.

8. As a user, I want `validate` to print normalized details and field-specific validation errors, so that I can check invoice payment details without generating a QR.

9. As a user, I want `from-text` to read copied invoice text from a file or stdin, so that the tool can suggest payment details without requiring each field manually.

10. As a user, I want flags to override `from-text` suggestions, so that I can provide missing or ambiguous fields without an edit wizard.

11. As a user, I want `from-pdf` to extract text from a PDF and reuse the same suggestion and confirmation flow, so that normal invoice PDFs can produce payment QR codes.

12. As a user, I want output format inferred from `.png` or `.svg`, so that common usage stays simple.

13. As a user, I want existing output files protected by default, so that invoice artifacts are not overwritten accidentally.

14. As a user, I want `--force` to overwrite an existing output file intentionally, so that rerunning a known command is possible.

15. As a user, I want EPC Version 2 payloads with optional BIC, so that Belgian banking apps can scan the QR without requiring obsolete BIC input.

16. As a maintainer, I want PDF/OCR/AI extraction outside the trusted path, so that future helpers can improve suggestions without weakening payment generation safety.

## Implementation Decisions

- Build a Go CLI binary named `invoiceqr`.

- Implement commands: `generate`, `validate`, `from-text`, and `from-pdf`.

- Use Kong for CLI parsing, with typed command structs close to validated payment-detail inputs.

- Use EPC/SEPA Credit Transfer QR payloads with service tag `BCD`, payment type `SCT`, currency `EUR`, and version `002`.

- Leave BIC empty by default, while allowing an optional BIC field for compatibility.

- Use UTF-8 payload charset.

- Preserve deterministic LF line endings and required blank lines in EPC payloads.

- Leave EPC payment purpose empty in v1 and do not expose a purpose flag.

- Encode a valid Belgian Structured Reference as structured remittance information.

- Encode all other accepted references as Unstructured Remittance Information.

- Never set both structured and unstructured remittance information in one payload.

- Reject values that look like Belgian Structured References when their checksum is invalid.

- Accept valid SEPA IBANs, not only Belgian IBANs.

- Parse amounts without floats; normalize accepted values to exactly two decimal places.

- Reject zero, negative, over-precision, and malformed amounts.

- Infer output format from `--out` extension; require `--format` only for unusual names or future stdout support.

- Refuse to overwrite existing output unless `--force` is set.

- Render QR codes using `github.com/piglig/go-qr`.

- Keep QR styling compatibility-focused: black on white, quiet zone enabled, medium error correction, no branding controls.

- Use `pdftotext` for v1 PDF text extraction.

- Reuse the same deterministic parser for `from-text` and `from-pdf` after PDF text extraction.

- Make `from-text` and `from-pdf` conservative: if multiple plausible IBANs or amounts are found, require explicit flags rather than guessing.

- Keep trusted core logic separate from extraction helpers.

- Major modules:
  - EPC payload builder: deterministic payload construction from Confirmed Payment Details.
  - Validation: IBAN, amount, Belgian Structured Reference, and remittance limits.
  - QR rendering: PNG/SVG output with compatibility defaults.
  - Extraction: deterministic text parser and `pdftotext` wrapper that produce Suggested Payment Details only.
  - Confirmation: payment-detail display and yes/no prompt.
  - CLI wiring: Kong commands, flag overrides, output behavior, and exit codes.

## Testing Decisions

- Test external behavior and deterministic outputs rather than CLI framework internals.

- Add exact-string EPC payload tests, including empty BIC and purpose lines.

- Add Belgian Structured Reference tests for valid references, invalid checksums, malformed syntax, and unstructured fallback.

- Add IBAN validation tests for valid SEPA IBANs, invalid checksum, invalid characters, and spacing normalization.

- Add amount tests for integer, one-decimal, two-decimal, over-precision, zero, negative, and malformed input.

- Add output-format tests for `.png`, `.svg`, unknown extension, and `--format` override.

- Add extraction tests for clear invoice text, missing fields, multiple IBANs, multiple amounts, flag overrides, and Belgian structured-reference detection.

- Add focused confirmation behavior tests where feasible with injected input/output.

- Run `go test ./...` and `go build ./...` before handoff.

## Out of Scope

- OCR.

- LLM or AI-based extraction.

- Python extraction helpers.

- UBL/Peppol XML invoice support.

- QR styling, logos, colors, or branding.

- Shell completions.

- Config files or persistent profiles.

- Network calls or live banking validation.

- Trusting PDF/text extraction without explicit confirmation.

## Further Notes

- Future OCR, Python, or LLM helpers may suggest fields, but must remain outside the trusted payment-generation path.

- Future UBL/Peppol XML support can provide higher-quality Suggested Payment Details, but still must pass deterministic validation and confirmation.

- The repository glossary distinguishes Payment Details, Suggested Payment Details, Confirmed Payment Details, Manual Payment Details, Belgian Structured Reference, and Unstructured Remittance Information.

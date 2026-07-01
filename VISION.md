# Vision

invoiceqr is a deterministic Go CLI for Belgian-compatible SEPA/EPC payment QR codes. It should keep payment generation validated, explicit, and agent-safe, with extraction and suggestions treated only as untrusted candidates until confirmed.

## Merge by Default

- Bug fixes in validation, EPC payload construction, QR rendering, dry-run plans, or output safety.
- Small CLI, JSON envelope, docs, and examples improvements that preserve deterministic behavior.
- Text/PDF suggestion improvements that keep confirmation boundaries and evidence clear.
- Tests for Belgian structured references, IBANs, amounts, overwrite rules, and artifact formats.
- Agent Context improvements that help callers inspect and override suggestions safely.

## Needs Sign-Off

- Built-in AI, OCR, or external service dependencies.
- Changes that let Suggested Payment Details produce QR artifacts without explicit confirmation.
- Relaxing validation, checksum handling, symlink refusal, or overwrite safety.
- New payment schemes, countries, QR standards, or remittance semantics.
- Handling invoice text in ways that expand privacy exposure without explicit review.

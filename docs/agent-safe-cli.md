---
summary: "Agent-safe invoiceqr command sequences for JSON validation, dry-run plans, suggestion evidence, confirmation, and QR artifact generation."
read_when: "Using invoiceqr from an agent, automation, or scripted workflow; changing JSON output, dry-run behavior, confirmation policy, or suggested payment details."
---

# Agent-Safe CLI Usage

## Summary

Use JSON modes to inspect Payment Details, Suggested Payment Details, Remittance Reference classification, and Payment Artifact Plan data before any QR artifact is written. Text, PDF, OCR, and AI helpers may produce Suggested Payment Details only; they must not create Confirmed Payment Details or skip explicit confirmation.

## Read when

Read when using `invoiceqr` from an agent, automation, or scripted workflow, or when changing JSON output, dry-run behavior, confirmation policy, or suggested payment details.

## Rules

- `validate --json` checks Manual Payment Details and prints parseable JSON without writing a QR artifact.
- `generate --dry-run --json` builds a Payment Artifact Plan only. The plan is no-write metadata, not confirmation.
- `generate --json` is the real Manual Payment Details QR write path. Without `--yes`, confirmation text is written to stderr so stdout stays parseable JSON.
- `from-text --dry-run --json` and `from-pdf --dry-run --json` print Suggested Payment Details with `value`, `source`, and `evidence`.
- Suggested Payment Details from text, PDF, OCR, or AI still require explicit confirmation before QR output.
- Do not treat evidence snippets, successful validation, or a Payment Artifact Plan as user approval.

## Manual Details Sequence

Validate candidate Manual Payment Details:

```sh
go run ./cmd/invoiceqr validate --payee "ACME BV" --iban BE68539007547034 --amount 42.50 --reference INV-2026-001 --json
```

Build a no-write Payment Artifact Plan:

```sh
go run ./cmd/invoiceqr generate --payee "ACME BV" --iban BE68539007547034 --amount 42.50 --reference INV-2026-001 --out invoice.svg --dry-run --json
```

Show the Payment Details and Payment Artifact Plan to the user. After explicit user approval, write the QR artifact:

```sh
go run ./cmd/invoiceqr generate --payee "ACME BV" --iban BE68539007547034 --amount 42.50 --reference INV-2026-001 --out invoice.svg --json
```

For already-known Manual Payment Details in trusted automation, `generate --yes --json` may skip the prompt. Do not use `--yes` for unconfirmed Suggested Payment Details.

## Suggested Details Sequence

Inspect copied invoice text without writing a QR artifact:

```sh
go run ./cmd/invoiceqr from-text invoice.txt --out invoice.svg --dry-run --json
```

Inspect PDF-extracted text evidence without PDF coordinates:

```sh
go run ./cmd/invoiceqr from-pdf invoice.pdf --out invoice.svg --dry-run --json
```

If a field is ambiguous or missing, supply an explicit override and inspect the source marker:

```sh
go run ./cmd/invoiceqr from-text invoice.txt --amount 42.50 --reference INV-2026-001 --out invoice.svg --dry-run --json
```

Show the Suggested Payment Details, evidence, Remittance Reference, and any Payment Artifact Plan to the user. To write a QR artifact from the suggestion path, run the non-dry-run command and require the user to approve the displayed Payment Details at the prompt; after approval, those become Confirmed Payment Details for generation:

```sh
go run ./cmd/invoiceqr from-text invoice.txt --out invoice.svg
go run ./cmd/invoiceqr from-pdf invoice.pdf --out invoice.svg
```

## JSON Streams

JSON modes reserve stdout for one parseable envelope with `success`, `data`, and `error`. When `generate --json` asks for confirmation, the human-readable Payment Details and prompt go to stderr. Agents should parse stdout only, inspect stderr for user interaction, and treat a nonzero exit as failed output generation.

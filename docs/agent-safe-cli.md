---
summary: "Agent-safe invoiceqr command sequences for JSON validation, dry-run plans, Agent Context recovery, confirmation, and QR artifact generation."
read_when: "Using invoiceqr from an agent, automation, or scripted workflow; changing JSON output, Agent Context recovery, dry-run behavior, confirmation policy, or suggested payment details."
---

# Agent-Safe CLI Usage

## Summary

Use JSON modes to inspect Payment Details, Suggested Payment Details, Agent Context, Remittance Reference classification, and Payment Artifact Plan data before any QR artifact is written. Text, PDF, OCR, AI helpers, and Calling Agents may produce Suggested Payment Details or explicit overrides only; they must not create Confirmed Payment Details or skip explicit confirmation.

## Read when

Read when using `invoiceqr` from an agent, automation, or scripted workflow, or when changing JSON output, dry-run behavior, confirmation policy, or suggested payment details.

## Rules

- `validate --json` checks Manual Payment Details and prints parseable JSON without writing a QR artifact.
- `generate --dry-run --json` builds a Payment Artifact Plan only. The plan is no-write metadata, not confirmation.
- `generate --json` is the real Manual Payment Details QR write path. Without `--yes`, confirmation text is written to stderr so stdout stays parseable JSON.
- `from-text --dry-run --json` and `from-pdf --dry-run --json` print Suggested Payment Details with `value`, `source`, and `evidence`, plus compact Agent Context for inspection.
- Incomplete suggestion dry-runs may return `success: false` with partial `data`; agents should inspect missing or ambiguous fields and call the CLI again with explicit overrides.
- Suggested Payment Details from text, PDF, OCR, AI, or a Calling Agent still require explicit confirmation before QR output.
- Do not treat evidence snippets, Agent Context, successful validation, or a Payment Artifact Plan as user approval.

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

Recovery loop for a Calling Agent:

1. Run `from-text` or `from-pdf` with `--dry-run --json`.
2. If `success` is `false` and `error.code` is `incomplete_suggestion`, inspect `data.missing_fields`, `data.ambiguous_fields`, `data.suggestions`, and `data.agent_context`.
3. Infer only explicit override values from the evidence; do not treat Agent Context as approval.
4. Re-run the same dry-run with overrides such as `--payee`, `--amount`, `--iban`, or `--reference`.
5. Show the resulting Payment Details and plan to the user before any non-dry-run QR write.

Incomplete suggestion JSON still has partial `data`:

```json
{
  "success": false,
  "data": {
    "suggestions": {
      "payee": { "value": "", "source": "" },
      "iban": {
        "value": "BE68 5390 0754 7034",
        "source": "text",
        "evidence": "Creditor footer ... IBAN BE68 5390 0754 7034"
      },
      "amount": { "value": "", "source": "" },
      "reference": {
        "value": "+++123/4567/89002+++",
        "source": "text",
        "evidence": "+++123/4567/89002+++"
      }
    },
    "missing_fields": ["payee", "amount"],
    "agent_context": {
      "source_text_hash": "sha256:...",
      "observed_lines": [
        { "kind": "payment_instruction", "line": 3, "text": "Please pay using the details below." },
        { "kind": "iban_context", "line": 10, "text": "Creditor footer ... IBAN BE68 5390 0754 7034" }
      ],
      "candidates": {
        "iban": [
          {
            "value": "BE68 5390 0754 7034",
            "normalized": "BE68539007547034",
            "evidence": "Creditor footer ... IBAN BE68 5390 0754 7034",
            "line": 10
          }
        ],
        "reference": [
          {
            "value": "+++123/4567/89002+++",
            "normalized": "+++123/4567/89002+++",
            "evidence": "+++123/4567/89002+++",
            "line": 8,
            "kind": "structured"
          }
        ]
      }
    }
  },
  "error": {
    "code": "incomplete_suggestion",
    "field": "payee",
    "message": "required"
  }
}
```

After explicit overrides, the dry-run can validate and add `data.plan`:

```sh
go run ./cmd/invoiceqr from-text invoice.txt --payee "Ecopower cv" --amount 87.65 --out invoice.svg --dry-run --json
```

When compact Agent Context is not enough for an agent or debugging workflow, request full extracted text explicitly. Full text may contain personal, contract, or customer data and should not be logged casually:

```sh
go run ./cmd/invoiceqr from-text invoice.txt --out invoice.svg --dry-run --json --full-text
go run ./cmd/invoiceqr from-pdf invoice.pdf --out invoice.svg --dry-run --json --full-text
```

Show the Suggested Payment Details, evidence, Remittance Reference, and any Payment Artifact Plan to the user. To write a QR artifact from the suggestion path, run the non-dry-run command and require the user to approve the displayed Payment Details at the prompt; after approval, those become Confirmed Payment Details for generation:

```sh
go run ./cmd/invoiceqr from-text invoice.txt --out invoice.svg
go run ./cmd/invoiceqr from-pdf invoice.pdf --out invoice.svg
```

## JSON Streams

JSON modes reserve stdout for one parseable envelope with `success`, `data`, and `error`. Suggestion dry-runs may include partial `data` on failure so agents can recover with overrides; `validate` and `generate` failures keep data null. When `generate --json` asks for confirmation, the human-readable Payment Details and prompt go to stderr. Agents should parse stdout only, inspect stderr for user interaction, and treat a nonzero exit as failed output generation.

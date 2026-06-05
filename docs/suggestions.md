---
summary: "How invoice text becomes conservative Suggested Payment Details and Agent Context before validation, confirmation, and QR output."
read_when: "Changing from-text, from-pdf, suggestion parsing, Agent Context, ambiguity handling, overrides, amount parsing, or confirmation policy."
---

# Suggestions

## Summary

`from-text` reads copied invoice text from a file or stdin, while `from-pdf` extracts text with local `pdftotext`. Both commands ask the Suggestion module for Suggested Payment Details and Agent Context. The module extracts conservative payee, IBAN, amount, and remittance-reference candidates, applies explicit CLI flag overrides, then sends complete suggestions through deterministic validation and mandatory confirmation before QR output.

## Read when

Read when changing `from-text`, `from-pdf`, suggestion parsing, Agent Context, amount parsing, IBAN/reference ambiguity handling, explicit flag overrides, stdin/file input, or confirmation policy for suggested details.

## Flow

- CLI adapters read text from a file, stdin, or `pdftotext` PDF extraction.
- The Suggestion module extracts candidates and reports required-field errors for missing details.
- Explicit flags override suggested fields.
- Multiple plausible IBANs or amounts are ambiguity errors unless flags resolve them.
- Dry-run JSON reports each suggested field with `value`, `source`, and a short text evidence snippet. Explicit flags use `source: "override"` and do not get text evidence.
- Dry-run JSON includes compact Agent Context for Calling Agents. Agent Context contains typed candidates, observed source lines with coarse kinds and extracted-text line numbers, and a source text hash.
- Failed dry-run suggestions may return partial data with `success: false`, a recoverable `incomplete_suggestion` error, missing or ambiguous field lists, partial suggestions, and Agent Context. They do not include a Payment Artifact Plan unless all required Payment Details validate.
- Full extracted text is opt-in with an explicit flag, because invoice text may contain personal, contract, or customer data.
- Suggested Payment Details are validated by the trusted payment-generation path.
- Suggested details always require confirmation before writing a QR artifact.

## Dry-run JSON

Suggestion dry-runs return the normal CLI envelope: `success`, `data`, and `error`.

`data.suggestions` contains selected field suggestions:

```json
{
  "payee": { "value": "ACME BV", "source": "text", "evidence": "Payee: ACME BV" },
  "iban": { "value": "BE68 5390 0754 7034", "source": "text", "evidence": "IBAN: BE68 5390 0754 7034" },
  "amount": { "value": "42.50", "source": "text", "evidence": "Amount: EUR 42.50" },
  "reference": { "value": "INV-2026-001", "source": "text", "evidence": "Reference: INV-2026-001" }
}
```

Explicit flags use `source: "override"` and omit `evidence`.

`data.agent_context` has this final shape:

```json
{
  "source_text_hash": "sha256:...",
  "full_text": "only present with --full-text",
  "observed_lines": [
    { "kind": "payment_instruction", "line": 3, "text": "Please pay using the details below." }
  ],
  "candidates": {
    "payee": [{ "value": "ACME BV", "evidence": "Payee: ACME BV", "line": 1 }],
    "iban": [{ "value": "BE68 5390 0754 7034", "normalized": "BE68539007547034", "evidence": "IBAN: BE68 5390 0754 7034", "line": 2 }],
    "amount": [{ "value": "EUR 42.50", "normalized": "42.50", "evidence": "Amount: EUR 42.50", "line": 3 }],
    "reference": [{ "value": "+++123/4567/89002+++", "normalized": "+++123/4567/89002+++", "evidence": "+++123/4567/89002+++", "line": 4, "kind": "structured" }]
  }
}
```

`full_text` is omitted unless `--full-text` is set. Empty candidate groups may be omitted by JSON encoding.

Incomplete suggestions return `success: false`, `error.code: "incomplete_suggestion"`, partial `data.suggestions`, `missing_fields`, `ambiguous_fields`, and Agent Context. They omit `data.plan`. Complete valid suggestions return `success: true` and include `data.plan` with validated Payment Details, EPC data, and output metadata.

## Safety

PDF extraction, copied text, OCR, AI, and Calling Agents may produce Suggested Payment Details or Manual Payment Details overrides only. They must not create Confirmed Payment Details, skip validation, skip confirmation, or write QR output without the same deterministic payment-generation path used for manual details.

Agent Context is evidence for inspection, not approval. A Calling Agent may use Agent Context to infer an override, but the CLI still validates the resulting Payment Details and requires confirmation before QR output.

## Amounts

Suggested amounts are parsed as complete numeric tokens. Plain decimals such as `42.50` and `42,50` are accepted, and common thousands formats such as `1.234,56`, `1,234.56`, and `1 234,56` normalize to `1234.56`. Regular, non-breaking, and narrow non-breaking spaces are accepted as grouping whitespace.

When PDF text extraction splits table cells across lines, a preferred label such as `Total amount to pay` may use the next non-empty currency-bearing line as its amount candidate. Preferred amount labels are used before generic `Total` lines so detail-table totals do not make the payable amount ambiguous.

Malformed grouped values are ignored as amount candidates rather than truncated. This prevents an invoice amount such as `1.234,567` from becoming `1.23`.

## Payees

Suggested payees prefer explicit labels such as `Payee:` or `Supplier:`. When an invoice footer combines the creditor name and `IBAN` on one line, the legal-entity name before the address and IBAN may be used as a conservative payee candidate. This creditor-IBAN inference requires an `IBAN` marker followed by an IBAN-shaped value, strips leading creditor/payee labels, and scans dash-separated segments so footer prefixes are ignored without breaking hyphenated legal names.

## Stdin Confirmation

When `from-text` reads invoice text from stdin, confirmation reads from the terminal instead of the already-consumed invoice stream. This keeps piped invoice text outside the trusted path while still requiring an explicit interactive confirmation.

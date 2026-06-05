---
summary: "Focused PRD for Agent Context in suggestion dry-run JSON."
read_when: "Breaking Agent Context work into issues; changing suggestion JSON, incomplete suggestion recovery, or calling-agent workflows."
---

# PRD: Agent Context for Invoice Suggestions

## Summary

Agent Context is implemented in `from-text` and `from-pdf` dry-run JSON so a Calling Agent can inspect incomplete or ambiguous Suggested Payment Details, infer explicit overrides from source evidence, and re-run validation without `invoiceqr` embedding AI provider calls.

## Read when

Read when planning, implementing, testing, or breaking down Agent Context, suggestion dry-run JSON, incomplete suggestion recovery, or calling-agent workflows.

## Problem Statement

Invoice PDFs use many layouts and languages. `pdftotext` can extract useful text, but conservative field parsers still miss values such as a payee in a footer or an amount embedded in a payment sentence. Hard-coding more invoice layouts will not scale, and embedding Claude, Codex, OpenAI, Anthropic, or local model calls in the CLI would add provider lock-in, subscription assumptions, API keys, and privacy concerns.

Calling Agents already have model reasoning available outside the CLI. They need structured, bounded source evidence from `invoiceqr` so they can inspect what was extracted, supply explicit overrides, and let the CLI keep deterministic validation and confirmation as the trusted boundary.

## Solution

`from-text` and `from-pdf` dry-run JSON include Agent Context. Agent Context exposes payment-oriented source evidence, typed candidates, field status, and an opt-in full-text view. The Suggestion module owns this data so file text and PDF text share the same behavior.

`invoiceqr` remains model-agnostic. It does not call AI providers. A Calling Agent may use Agent Context to infer missing or ambiguous fields, but it must pass those values back as explicit overrides and re-run dry-run validation before any QR artifact is generated.

## User Stories

1. As a Calling Agent, I want compact Agent Context in dry-run JSON, so that I can inspect source evidence without scraping terminal output or rerunning text extraction myself.

2. As a Calling Agent, I want incomplete suggestion failures to include partial data, so that I can recover by supplying explicit overrides instead of starting over.

3. As a Calling Agent, I want typed candidates with source values, normalized values, evidence, and line numbers, so that I can explain why a field was selected or rejected.

4. As a Calling Agent, I want broader observed source lines with coarse kinds, so that I can infer fields the conservative parser did not select.

5. As a user, I want full extracted text to be opt-in, so that normal agent output does not casually expose all personal or invoice data.

6. As a maintainer, I want Agent Context built in the Suggestion module, so that `from-text`, `from-pdf`, and future extraction adapters share one tested contract.

7. As a maintainer, I want `invoiceqr` to avoid built-in model provider calls, so that the CLI remains free, local, and independent of subscription or API-key setup.

8. As a user, I want confirmation rules unchanged, so that no source evidence, Agent Context, or Calling Agent inference can write a QR artifact without explicit confirmation.

## Implementation Decisions

- Extend existing `from-text` and `from-pdf` dry-run JSON; do not add a new command.

- Add an explicit full-text opt-in flag for suggestion dry-runs. Default output includes compact Agent Context only.

- The Suggestion module owns Agent Context generation. CLI JSON formatting serializes the report but does not decide source evidence.

- Agent Context includes a source text hash by default.

- Agent Context includes `observed_lines` with coarse stable kinds, extracted-text line numbers, and text. Current kinds are `payment_instruction`, `amount_context`, `iban_context`, `reference_context`, `payee_context`, `document_header`, and `document_footer`.

- Observed lines are broader than typed parser candidates. They may include payment instructions, nearby creditor/footer/header lines, and other payment-adjacent evidence that a Calling Agent can reason over.

- Typed candidates stay conservative. They include only values the CLI can parse or classify as candidates for a specific field.

- Typed candidate objects include source `value`, optional deterministic `normalized` value, `evidence`, and extracted-text `line`. Remittance reference candidates may include `kind: "structured"` or `kind: "unstructured"`.

- Selected suggestions keep the existing `value`, `source`, and `evidence` shape for easy access.

- Incomplete suggestion dry-runs return `success: false` with partial `data`, a recoverable `incomplete_suggestion` error, missing or ambiguous field lists, partial suggestions, candidates, and Agent Context.

- Incomplete suggestion dry-runs do not include a Payment Artifact Plan.

- Complete valid suggestion dry-runs return `success: true` with suggestions, candidates, Agent Context, and a Payment Artifact Plan.

- `full_text` is omitted unless `--full-text` is set. Default Agent Context uses observed lines, candidates, and `source_text_hash` only.

- `validate` and `generate` error envelopes keep null data. Partial failure data applies only to suggestion dry-runs.

- Normal non-dry-run confirmation prompts remain compact and show Payment Details only. Agent Context is dry-run JSON only.

- A Calling Agent should re-run dry-run JSON with explicit overrides before generating a QR artifact.

- `invoiceqr` does not embed AI provider calls, subscription-backed model calls, local model orchestration, OCR, or image rendering in this implementation slice.

## Testing Decisions

- Added Suggestion module tests for Agent Context generation from invoice text.

- Added tests for observed line kinds, line numbers, deduplication, and source text hash.

- Added tests for typed candidate objects with source value, normalized value, evidence, and line number.

- Added tests for incomplete suggestion dry-runs returning partial data and `incomplete_suggestion`.

- Added tests proving incomplete suggestions omit the Payment Artifact Plan.

- Added tests proving complete suggestions include suggestions, Agent Context, and a Payment Artifact Plan.

- Added CLI JSON tests for the full-text opt-in flag.

- Added a regression fixture based on Ecopower-style invoice text: IBAN and Belgian Structured Reference are detected, Agent Context includes the payment instruction and creditor footer lines, and missing fields can be resolved by explicit overrides.

- Keep tests deterministic. Do not use live AI, OCR, network calls, or external model providers.

- Run `go test ./...` and `go build ./...` before handoff.

## Out of Scope

- Built-in AI provider calls.

- Claude, Codex, ChatGPT, Anthropic, OpenAI API, or local-model integrations inside `invoiceqr`.

- OCR and image rendering.

- UBL/Peppol XML support.

- New commands.

- Changes to QR generation, EPC payload construction, validation rules, or confirmation policy.

- Full invoice text in default JSON output.

## Further Notes

- This PRD follows ADR 0005: expose Agent Context instead of calling AI.

- Agent Context is evidence, not approval. Only Confirmed Payment Details may produce a QR artifact.

- Future OCR or UBL adapters can feed the same Suggestion module and Agent Context contract, but they remain outside this implementation slice.

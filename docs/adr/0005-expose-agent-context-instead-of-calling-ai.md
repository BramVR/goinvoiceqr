---
summary: "ADR for exposing Agent Context to calling agents instead of embedding AI provider calls in invoiceqr."
read_when: "Changing agent workflows, suggestion JSON, AI/OCR scope, extraction fallback behavior, or provider integration."
---

# Expose Agent Context Instead of Calling AI

`invoiceqr` stays model-agnostic: it does not call Claude, Codex, OpenAI, Anthropic, or local model providers to interpret invoice text. Instead, `from-text` and `from-pdf` dry-run JSON exposes Agent Context with bounded source lines, typed candidates, missing or ambiguous field status, a source text hash, and optional full extracted text so a Calling Agent can inspect incomplete Suggested Payment Details and supply explicit overrides. This keeps subscriptions, API keys, and provider policy outside the CLI while preserving the trusted boundary: Agent Context is evidence, not approval, and only deterministically validated and explicitly confirmed Payment Details may produce a QR artifact.

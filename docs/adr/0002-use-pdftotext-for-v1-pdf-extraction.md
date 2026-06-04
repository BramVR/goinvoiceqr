---
summary: "ADR for using pdftotext as the v1 PDF extraction adapter feeding the Suggestion module."
read_when: "Changing from-pdf, PDF extraction, extraction adapters, or trusted payment-generation boundaries."
---

# Use pdftotext for V1 PDF Extraction

## Summary

`invoiceqr from-pdf` uses the local `pdftotext` binary for v1 PDF text extraction, then feeds that text into the same suggestion and confirmation path as `from-text`. PDF extraction is outside the trusted payment-generation path: it may suggest payment details, but deterministic validation and explicit confirmation remain required before QR generation.

## Read when

Read when changing `from-pdf`, PDF extraction, text extraction adapters, Suggested Payment Details, or trusted payment-generation boundaries.

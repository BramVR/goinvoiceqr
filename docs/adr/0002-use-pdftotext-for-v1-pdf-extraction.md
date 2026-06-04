# Use pdftotext for V1 PDF Extraction

`invoiceqr from-pdf` uses the local `pdftotext` binary for v1 PDF text extraction, then feeds that text into the same suggestion and confirmation path as `from-text`. PDF extraction is outside the trusted payment-generation path: it may suggest payment details, but deterministic validation and explicit confirmation remain required before QR generation.

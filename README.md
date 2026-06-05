# invoiceqr

![goinvoiceqr hero showing Belgian SEPA/EPC invoice payment QR generation](docs/assets/goinvoiceqr-hero.webp)

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![CLI](https://img.shields.io/badge/CLI-Kong-111827)](https://github.com/alecthomas/kong)
[![QR](https://img.shields.io/badge/QR-SVG%20%2B%20PNG-06B6D4)](https://github.com/piglig/go-qr)
[![Payments](https://img.shields.io/badge/Payments-SEPA%2FEPC-FACC15)](docs/adr/0001-use-epc-version-2-with-optional-bic.md)
[![PDF](https://img.shields.io/badge/PDF-pdftotext-64748B)](docs/adr/0002-use-pdftotext-for-v1-pdf-extraction.md)

`invoiceqr` is a Go CLI for Belgian-compatible SEPA/EPC payment QR codes.

The CLI implements payment-detail validation, deterministic EPC payload construction, QR rendering, QR artifact output policy, manual QR generation, and text/PDF-based payment-detail suggestions.

## Safety Model

`invoiceqr` keeps payment generation deterministic. Manual Payment Details are the values supplied directly through CLI flags such as `--payee`, `--iban`, `--amount`, and `--reference`. Suggested Payment Details are candidates extracted from copied invoice text or PDF text extraction. Confirmed Payment Details are validated details that the user explicitly approved before QR output.

Text extraction, PDF extraction, OCR, and AI may suggest values only. They must not bypass validation, confirmation, or the trusted payment-generation path. `generate --yes` is intended for already-known manual details. Suggested details from `from-text` and `from-pdf` always require explicit confirmation before output.

Agent and automation workflows should follow [agent-safe CLI usage](docs/agent-safe-cli.md) for JSON validation, no-write Payment Artifact Plans, Agent Context recovery, full-text privacy tradeoffs, and confirmation boundaries.

## Build

```sh
go build ./...
```

Build the runnable CLI binary:

```sh
go build -o invoiceqr ./cmd/invoiceqr
```

## Test

```sh
go test ./...
```

## Run

Show available commands:

```sh
go run ./cmd/invoiceqr --help
```

Validate manual payment details without writing a QR artifact:

```sh
go run ./cmd/invoiceqr validate --payee "ACME BV" --iban BE68539007547034 --amount 42.50 --reference INV-2026-001
```

Generate a QR artifact from manual payment details:

```sh
go run ./cmd/invoiceqr generate --payee "ACME BV" --iban BE68539007547034 --amount 42.50 --reference INV-2026-001 --out invoice.svg
go run ./cmd/invoiceqr generate --payee "ACME BV" --iban BE68539007547034 --amount 42.50 --reference INV-2026-001 --out invoice.png
go run ./cmd/invoiceqr generate --payee "ACME BV" --iban BE68539007547034 --amount 42.50 --reference INV-2026-001 --out invoice.svg --yes --json
go run ./cmd/invoiceqr generate --payee "ACME BV" --iban BE68539007547034 --amount 42.50 --reference INV-2026-001 --out invoice.svg --dry-run --json
```

Suggest payment details from copied invoice text:

```sh
go run ./cmd/invoiceqr from-text invoice.txt --out invoice.svg
pbpaste | go run ./cmd/invoiceqr from-text --out invoice.svg
go run ./cmd/invoiceqr from-text invoice.txt --out invoice.svg --dry-run --json
```

Suggest payment details from a PDF invoice:

```sh
go run ./cmd/invoiceqr from-pdf invoice.pdf --out invoice.svg
go run ./cmd/invoiceqr from-pdf invoice.pdf --out invoice.svg --dry-run --json
```

Override a suggested field when the invoice text is ambiguous or incomplete:

```sh
go run ./cmd/invoiceqr from-text invoice.txt --amount 42.50 --reference INV-2026-001 --out invoice.svg
```

`validate` prints normalized payment details and field-specific validation errors. `generate` validates manual payment details, asks for confirmation unless `--yes` is supplied, and writes a QR artifact. `from-text` and `from-pdf` suggest payment details and always require confirmation before output.

Pass `--json` to `validate` to print a machine-readable envelope with `success`, `data`, and `error` fields.

Pass `--json` to `generate` to print a machine-readable artifact result after the QR file is written. When confirmation is required, payment details and the prompt are written to stderr so stdout remains parseable JSON.

Pass `--dry-run --json` to `generate` to validate manual payment details, build the EPC payload, and preflight QR output without prompting or writing a file.

Pass `--dry-run --json` to `from-text` or `from-pdf` to print Suggested Payment Details with per-field source and evidence snippets plus compact Agent Context for incomplete or ambiguous recovery. Complete valid suggestions also include the same no-write Payment Artifact Plan used by `generate --dry-run --json`. Add `--full-text` only when the Calling Agent needs the whole extracted text; invoice text may contain personal, contract, or customer data.

## Output

`--out` is required for QR generation. The file extension selects the output format when it is `.svg` or `.png`; otherwise pass `--format svg` or `--format png`.

Existing output files are refused by default to avoid accidental replacement. Pass `--force` to overwrite an existing QR artifact. On Unix and Windows targets, symlink output paths are refused even with `--force`.

## PDF Extraction

`from-pdf` shells out to the local `pdftotext` command and reads its extracted text. Install Poppler or another package that provides `pdftotext` before using PDF suggestions. When `pdftotext` is missing, the CLI reports that dependency and asks you to install `poppler`/`pdftotext` and retry.

# invoiceqr

`invoiceqr` is a Go CLI for Belgian-compatible SEPA/EPC payment QR codes.

The current scaffold wires the planned command entrypoints and implements payment-detail validation, deterministic EPC payload construction, QR rendering, QR artifact output policy, manual QR generation, and text-based payment-detail suggestions.

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

```sh
go run ./cmd/invoiceqr --help
go run ./cmd/invoiceqr generate --payee "ACME BV" --iban BE68539007547034 --amount 42.50 --reference INV-1 --out invoice.svg
go run ./cmd/invoiceqr validate
go run ./cmd/invoiceqr from-text invoice.txt --out invoice.svg
go run ./cmd/invoiceqr from-pdf
```

`validate` prints normalized payment details and field-specific validation errors. `generate` validates manual payment details, asks for confirmation unless `--yes` is supplied, and writes a QR artifact. `from-text` suggests payment details from copied text and always requires confirmation before output. The `from-pdf` command implementation currently returns a placeholder error until its extraction module is implemented.

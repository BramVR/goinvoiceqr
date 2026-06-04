# invoiceqr

`invoiceqr` is a Go CLI for Belgian-compatible SEPA/EPC payment QR codes.

The current scaffold wires the planned command entrypoints and implements payment-detail validation, deterministic EPC payload construction, QR rendering, QR artifact output policy, and manual QR generation.

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
go run ./cmd/invoiceqr from-text
go run ./cmd/invoiceqr from-pdf
```

`validate` prints normalized payment details and field-specific validation errors. `generate` validates manual payment details, asks for confirmation unless `--yes` is supplied, and writes a QR artifact. The `from-text` and `from-pdf` command implementations currently return placeholder errors until their suggestion modules are implemented.

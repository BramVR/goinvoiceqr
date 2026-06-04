# invoiceqr

`invoiceqr` is a Go CLI for Belgian-compatible SEPA/EPC payment QR codes.

The current scaffold wires the planned command entrypoints and implements payment-detail validation. EPC payload construction and QR rendering are not implemented yet.

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
go run ./cmd/invoiceqr generate
go run ./cmd/invoiceqr validate
go run ./cmd/invoiceqr from-text
go run ./cmd/invoiceqr from-pdf
```

`validate` prints normalized payment details and field-specific validation errors. The other command implementations currently return placeholder errors until their payment-generation modules are implemented.

---
summary: "ADR for using Kong typed command structs for invoiceqr CLI parsing."
read_when: "Changing CLI command structure, flag parsing, help output, or command wiring."
---

# Use Kong for CLI Parsing

## Summary

`invoiceqr` uses Kong for command-line parsing so command definitions stay as typed Go structs close to the payment-detail inputs they validate. This gives better help and subcommand ergonomics than the standard library while avoiding the larger command scaffolding style of Cobra.

## Read when

Read when changing command structs, flags, subcommands, help output, or the CLI parsing dependency.

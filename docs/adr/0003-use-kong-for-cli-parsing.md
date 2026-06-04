# Use Kong for CLI Parsing

`invoiceqr` uses Kong for command-line parsing so command definitions stay as typed Go structs close to the payment-detail inputs they validate. This gives better help and subcommand ergonomics than the standard library while avoiding the larger command scaffolding style of Cobra.

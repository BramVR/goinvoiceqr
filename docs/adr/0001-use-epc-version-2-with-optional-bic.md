# Use EPC Version 2 With Optional BIC

Belgian banking apps support EPC QR version `002`, where BIC is optional for SEPA credit transfers. `invoiceqr` generates version `002` payloads with an empty BIC line by default, while keeping BIC as an optional field for compatibility with payees or banks that still provide it.

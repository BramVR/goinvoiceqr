# Invoice QR

This context covers turning invoice payment information into Belgian-compatible SEPA/EPC payment QR codes.

## Language

**Payment Details**:
The payee name, IBAN, amount, and remittance reference needed to describe one SEPA credit transfer.
_Avoid_: Invoice data, QR data

**Suggested Payment Details**:
Payment details inferred from copied text, PDF text extraction, or another helper before user confirmation.
_Avoid_: Detected details, parsed payment

**Confirmed Payment Details**:
Payment details that have passed deterministic validation and explicit user confirmation.
_Avoid_: Approved suggestion, trusted extraction

**Manual Payment Details**:
Payment details supplied field-by-field by the caller instead of inferred from document text.
_Avoid_: Raw CLI args

**Remittance Reference**:
The payment communication carried with the transfer, either a Belgian structured reference or unstructured remittance information.
_Avoid_: Message, note, description

**Belgian Structured Reference**:
A checksum-protected Belgian creditor reference written in the standard `+++123/1234/12345+++` style. `invoiceqr` also accepts the slash-after-plus variant `+++/123/1234/12345+++`.
_Avoid_: OGM, structured communication

**Unstructured Remittance Information**:
Free-form payment communication used when no valid Belgian structured reference exists.
_Avoid_: Plain reference, fallback message

## Example Dialogue

Dev: "The PDF extractor found suggested payment details."
Domain expert: "Show them to the user first. Only confirmed payment details may produce a QR code."

Dev: "The invoice has `+++123/1234/12345+++`."
Domain expert: "That is a Belgian structured reference, so validate its checksum before confirmation."

Dev: "The user entered a normal invoice number as the remittance reference."
Domain expert: "Treat it as unstructured remittance information. Do not also set a structured reference."

Dev: "`generate --yes` skips the prompt after validating manual payment details."
Domain expert: "That is fine for manual payment details. Suggested payment details still need explicit confirmation."

Dev: "The reference looks like `+++123/1234/12345+++` but the checksum is wrong."
Domain expert: "Reject it. Do not silently downgrade a malformed Belgian structured reference to unstructured remittance information."

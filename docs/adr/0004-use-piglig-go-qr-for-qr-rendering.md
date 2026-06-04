# Use piglig/go-qr for QR Rendering

`invoiceqr` uses `github.com/piglig/go-qr` for QR encoding and rendering because it supports both PNG and SVG output directly from one Go dependency. The project keeps QR styling minimal and compatibility-focused: black modules, white background, normal quiet zone, and medium error correction by default.

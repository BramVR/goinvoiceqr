---
summary: "ADR for using piglig/go-qr to render compatibility-focused PNG and SVG QR artifacts."
read_when: "Changing QR rendering dependency, output formats, quiet zone, styling, or error correction behavior."
---

# Use piglig/go-qr for QR Rendering

## Summary

`invoiceqr` uses `github.com/piglig/go-qr` for QR encoding and rendering because it supports both PNG and SVG output directly from one Go dependency. The project keeps QR styling minimal and compatibility-focused: black modules, white background, normal quiet zone, and medium error correction by default. Output writes refuse symlink paths even when `--force` is supplied.

## Read when

Read when changing QR rendering dependency, output formats, quiet-zone behavior, styling defaults, or error correction policy.

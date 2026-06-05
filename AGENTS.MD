# Repository Guidelines

## Project Structure & Modules
- Go CLI project for `invoiceqr`.
- Keep payment-generation logic deterministic in Go.
- Do not put AI/OCR/PDF extraction in the trusted payment-generation path.

## Build, Test, Run
- Use Go modules.
- Test: `go test ./...`.
- Build: `go build ./...` unless a narrower package target exists.
- Run CLI locally with `go run ...` until a binary target exists.

## Docs
- Docs live in `docs/`.
- Before coding or PR, run `docs-list` and read relevant docs by `read_when`.
- Docs should include `summary` and `read_when` front matter plus `## Summary` and `## Read when` headings.
- Update docs for user-visible behavior changes.

## Coding Style & Naming
- Keep changes small and scoped.
- Prefer typed, deterministic core logic.
- Validate inputs before generating payment output.

## Testing Guidelines
- Add focused Go tests for new behavior.
- Prefer deterministic tests over live or external-service checks.
- Run `go test ./...` before handoff.

## Commit & PR Guidelines
- Use `committer` for commits: `committer "Commit message" path/to/file [path/to/other]`; list explicit paths, never `.`.

## Agent Notes
- Before generating payment output, show the detected/generated details and require explicit confirmation.

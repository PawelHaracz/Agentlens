# DevOps: Git Hooks (Lefthook)

Git hooks are managed via lefthook, checked into the repo as `lefthook.yml`. Install via `make hooks`.

### Pre-Commit Hooks (Parallel)
`pre-commit.go-fmt`: runs `test -z "$(gofmt -l .)"`, fails with "Go files need formatting. Run: make format" if any file is not gofmt-formatted. `pre-commit.go-lint`: runs `golangci-lint run`. `pre-commit.web-lint`: runs `cd web && bun run type-check`. All three run in parallel. Source: lefthook.yml.

### Commit-Msg Hook
`strip-ai-coauthors` runs first (priority 1), then `commitlint` validates against `web/commitlint.config.ts` with `bunx commitlint --config commitlint.config.ts --edit {1}`. Source: lefthook.yml.

### Pre-Push Hook
`go-test`, `web-test` (parallel, priority 1), and `arch-test` (priority 2). Blocks push if any fails. Source: lefthook.yml.

### Install Hooks via `make hooks`
After cloning, run `rtk make hooks` once to install lefthook-managed hooks. Activation is explicit — not auto-run from `make all`. Sources: docs/developer-guide.md, ADR-002.

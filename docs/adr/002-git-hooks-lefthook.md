# ADR-002: Use lefthook for Git Hook Management

Date: 2026-04-11
Status: Accepted

## Context

No local quality gate exists. Contributors can push code that fails CI lint, tests, or build and only discover the failure after the push. Commit messages are inconsistent — no conventional commits format is enforced. The team needs hooks that are checked into the repo so every contributor gets them automatically, with a single activation command.

The repo already has bun + Node tooling in `web/` and installs Go tooling via `go install` in `make tools`. Any solution must fit these existing patterns without adding new runtimes.

## Decision

Use **lefthook** to manage git hooks, checked into the repo as `lefthook.yml`. Use **commitlint** (`@commitlint/cli` + `@commitlint/config-conventional`) for commit message validation, invoked via `bunx` from the existing bun installation.

Three hook stages:

- **pre-commit**: `gofmt` check + `golangci-lint` + TypeScript type-check (parallel, fast)
- **commit-msg**: commitlint enforcing conventional commits with optional scope
- **pre-push**: `make test` + `make web-test` (parallel) + `make arch-test` (sequential after)

Activation: `make hooks` (explicit, not auto-run from `make all`).

## Consequences

### Positive

- Hooks are version-controlled and consistent across all contributors.
- Single `make hooks` command activates everything.
- Parallel execution in lefthook keeps pre-commit fast.
- Conventional commits enforced from day one — enables automated changelogs later.
- `arch-test` local coverage closes a gap (not yet in CI).

### Negative / Trade-offs

- Contributors must run `make hooks` once after cloning — not zero-friction but close.
- lefthook binary must be installed (`go install`) — adds to onboarding setup.
- `@commitlint` packages add 2 devDependencies to `web/package.json`.

### Neutral

- Hooks mirror CI but don't replace it. CI remains the authoritative gate.
- Contributors can skip hooks with `--no-verify` or `LEFTHOOK=0` when needed.

## Alternatives considered

| Option | Why rejected |
| ------ | ------------ |
| **husky** | Requires Node.js for hook management itself; bun is present but husky's setup model (`prepare` script) conflicts with Go-first tooling pattern |
| **Plain shell scripts in `.githooks/`** | No parallel execution, brittle cross-platform behaviour, poor error output |
| **Shell regex for commit-msg** | Fragile pattern maintenance, terse error messages, hard to extend |
| **commitizen (Go binary)** | Less maintained, smaller ecosystem, fewer contributors familiar with it |

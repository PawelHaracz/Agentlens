# Git Hooks Design

**Date:** 2026-04-11  
**Status:** Approved  
**ADR:** [ADR-002](../../adr/002-git-hooks-lefthook.md)

---

## Problem

No local quality gate exists. Contributors can push code that fails CI (lint, tests, build) and only discover it after the push. Commit messages are inconsistent — no conventional commits enforcement. Hooks must work for every contributor without manual per-machine configuration beyond one setup command.

---

## Goals

1. Block bad commits locally before they reach CI.
2. Enforce conventional commit message format.
3. Run heavier checks (tests) before push, not on every commit.
4. Zero friction for contributors — one command to activate.
5. Hooks are checked into the repo so everyone gets them automatically.

---

## Architecture

```
repo root/
├── lefthook.yml              ← hook definitions (checked in)
├── commitlint.config.ts      ← conventional commits config (root level)
├── web/
│   └── package.json          ← devDependencies: @commitlint/cli, @commitlint/config-conventional
└── Makefile                  ← adds `hooks` target
```

### Hook framework: lefthook

- Single Go binary, no Node.js runtime required for hook management
- Config is a single `lefthook.yml` checked into the repo
- Parallel job execution within each hook stage
- Cross-platform (macOS, Linux, Windows)
- Installed via `go install` — consistent with existing `make tools` pattern

### Commit message validation: commitlint

- `@commitlint/cli` + `@commitlint/config-conventional` as `devDependencies` in `web/package.json`
- Invoked via `bunx commitlint` — bun already present in the repo
- `commitlint.config.ts` at repo root

---

## Hook Definitions

### pre-commit (fast — no tests)

Runs on every `git commit`. All three jobs execute in parallel.

| Job | Command | Purpose |
|-----|---------|---------|
| `go-fmt` | `gofmt -l .` | Fails if any Go files need formatting |
| `go-lint` | `golangci-lint run` | Go static analysis |
| `web-lint` | `cd web && bun run type-check` | TypeScript type checking |

If any job fails, the commit is blocked and the error is shown.

### commit-msg

Runs after pre-commit, validates the commit message.

```
bunx --cwd web commitlint --edit $1
```

**Allowed types:** `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `build`, `perf`, `revert`  
**Scope:** optional  
**Format:** `<type>[(scope)]: <subject>` — max 100 chars  

Valid examples:
```
feat: add kubernetes source plugin
feat(api): expose capabilities endpoint
fix(auth): correct lockout timer reset
chore: bump go version to 1.26.1
```

### pre-push (heavier — runs tests)

Runs on `git push`. Go test and web test run in parallel; arch-test runs after to avoid CGO conflicts.

| Job | Command | Priority |
|-----|---------|---------|
| `go-test` | `make test` | 1 (parallel) |
| `web-test` | `make web-test` | 1 (parallel) |
| `arch-test` | `make arch-test` | 2 (after above) |

---

## Developer Experience

### Activation (once per clone)

```bash
make hooks
```

Installs lefthook and activates all hooks. Must be run after cloning. Not auto-activated from `make all` to avoid surprising contributors.

### Skipping hooks

```bash
git commit --no-verify -m "wip: ..."   # skip pre-commit + commit-msg
git push --no-verify                    # skip pre-push
LEFTHOOK=0 git commit ...              # disable lefthook entirely
```

### lefthook.yml structure

```yaml
pre-commit:
  parallel: true
  commands:
    go-fmt:
      run: test -z "$(gofmt -l .)"
      fail_text: "Go files need formatting. Run: make format"
    go-lint:
      run: golangci-lint run
    web-lint:
      run: cd web && bun run type-check

commit-msg:
  commands:
    commitlint:
      run: bunx --cwd web commitlint --edit {1}

pre-push:
  commands:
    go-test:
      run: make test
      priority: 1
    web-test:
      run: make web-test
      priority: 1
    arch-test:
      run: make arch-test
      priority: 2
```

### commitlint.config.ts

```typescript
import type { UserConfig } from "@commitlint/types";

const config: UserConfig = {
  extends: ["@commitlint/config-conventional"],
  rules: {
    "subject-max-length": [2, "always", 100],
    "scope-case": [2, "always", "lower-case"],
    "type-enum": [
      2,
      "always",
      ["feat", "fix", "chore", "docs", "refactor", "test", "ci", "build", "perf", "revert"],
    ],
  },
};

export default config;
```

---

## Makefile Changes

```makefile
## hooks: Install git hooks via lefthook (run once after cloning)
hooks:
    go install github.com/evilmartians/lefthook@latest
    lefthook install
```

Also add lefthook to the `tools` target so `make tools` covers it.

---

## CI Relationship

Hooks mirror CI but do not replace it. CI remains the authoritative gate.

| Hook | CI equivalent |
|------|--------------|
| pre-commit: go-fmt | CI lint job: `make vet` |
| pre-commit: go-lint | CI lint job: `make lint` |
| pre-commit: web-lint | CI lint job: `make web-lint` |
| pre-push: go-test | CI test job: `make test-coverage` |
| pre-push: web-test | CI test-frontend job: `make web-test-coverage` |
| pre-push: arch-test | (not yet in CI — add as follow-up) |

Note: CI runs `test-coverage` (with coverage report); hooks run plain `make test` (faster, no coverage overhead).

---

## Docs Changes

`docs/developer-guide.md` gets a new **Git Hooks** section covering:
- What each hook does
- How to install (`make hooks`)
- How to skip when needed
- How to verify hooks are installed (`lefthook list`)

---

## Out of Scope

- `pre-receive` server-side hooks (requires GitHub Enterprise)
- Automatic hook activation on `git clone`
- Branch name validation
- Secret scanning in hooks (CodeQL workflow already handles this)

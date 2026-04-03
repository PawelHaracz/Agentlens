# Design: arch-go Architectural Rules

**Date:** 2026-04-03
**Status:** Approved

## Goal

Enforce the microkernel layer boundaries and code quality conventions at CI time using [arch-go](https://github.com/arch-go/arch-go), so architectural violations are caught before merge — not discovered during manual code review.

## Approach

Standalone CLI with `arch-go.yml` config at repo root. New `make arch-test` target added to `make all` pipeline.

## Rules

### Dependency Rules

These enforce the microkernel layer boundaries. The dependency graph has clear tiers:

```
[foundation] model, config, db          — no internal deps
[infrastructure] store, auth, service   — depend on foundation only
[core] kernel, discovery                — depend on foundation + infrastructure
[api] api                               — depends on core + infrastructure + kernel
[plugins] plugins/**                    — depend on kernel + foundation only
[entrypoint] cmd/**                     — composition root, may import anything
```

| Package | `shouldNotDependsOn` (internal) |
|---|---|
| `**.internal.model` | any `**.agentlens.**` internal package |
| `**.internal.config` | any `**.agentlens.**` internal package |
| `**.internal.service` | any `**.agentlens.**` internal package |
| `**.internal.kernel` | `**.internal.api`, `**.internal.auth`, `**.internal.server`, `**.plugins.**` |
| `**.internal.store` | `**.internal.api`, `**.internal.auth`, `**.internal.kernel`, `**.internal.server`, `**.plugins.**` |
| `**.plugins.**` | `**.internal.api`, `**.internal.auth`, `**.internal.server`, `**.cmd.**` |
| `**.internal.api` | `**.cmd.**`, `**.plugins.**` |

### Contents Rules

| Package | Rule |
|---|---|
| `**.internal.model` | `shouldNotContainInterfaces` — pure domain types |
| `**.internal.config` | `shouldNotContainInterfaces` — pure config structs |

### Functions Rules

Applied to `**.internal.**` and `**.plugins.**`:

| Constraint | Value |
|---|---|
| `maxParameters` | 5 |
| `maxReturnValues` | 3 |
| `maxLines` | 80 |
| `maxPublicFunctionPerFile` | 10 |

### Naming Rules

| Package | Rule |
|---|---|
| `**.plugins.**` | Structs implementing `Plugin` interface must have name ending with `Plugin` |

This is intentionally minimal — expand naming rules as new conventions emerge (see CLAUDE.md note).

## Integration

- `arch-go.yml` at repo root
- `make arch-test` runs `arch-go`
- `make all` order: `format → lint → test → arch-test → build`
- Thresholds: `compliance: 100`, `coverage: 80` (not all packages need rules, but all rules must pass)

## Files Changed

| File | Change |
|---|---|
| `arch-go.yml` | New — all rules |
| `Makefile` | Add `arch-test` target, add to `all` |
| `CLAUDE.md` | Document arch-go and naming rule expansion policy |

## Out of Scope

- Frontend architecture rules (separate sub-project: frontend unit tests)
- Go test wrapper for arch-go (may add later if programmatic access is needed)

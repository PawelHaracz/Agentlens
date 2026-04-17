# Backend Go: Conventions & Patterns

Project-specific Go conventions enforced by reviewers, lint, and pre-commit hooks. Applies to `internal/**` and `plugins/**`.

### `context.Context` as First Argument on I/O

Functions that perform I/O (store, discovery, handlers, plugins, HTTP, DB, external calls) must take `ctx context.Context` as the first parameter. Example: `func (s *UserStore) IncrementFailedAttempts(ctx context.Context, id string) error`. Sources: code-patterns (82+ sites across 24 files), CLAUDE.md.

### Use Request Context With Deadlines for DB and External I/O

DB pings, on-demand probes, and external fetches must accept/propagate `context.Context` with a timeout rather than `context.Background()`. Use `slog.InfoContext(r.Context(), ...)` for audit logs so they correlate with requests and traces. Source: PR reviews (#22 `dbPingFn`, #15 audit logs).

### Error Wrapping with `fmt.Errorf` and `%w`

Wrap errors with `fmt.Errorf("doing x: %w", err)` using an action-describing prefix. Use `errors.Is` / `errors.As` for type checks — never string-match on `err.Error()`. Example: `return fmt.Errorf("finding user for failed attempt increment: %w", err)`. Sources: code-patterns (142 occurrences across 36 files), CLAUDE.md.

### slog Structured Logging with Context Fields

All production logging uses `log/slog` with key-value structured pairs. Pass the context when available. Include `component` and `plugin` fields. The standard `log` package is reserved for examples/mock-agents only. Example: `slog.Info("http request", "method", r.Method, "path", r.URL.Path, "status", rw.status)`. Sources: code-patterns, CLAUDE.md, docs/developer-guide.md.

### No Panic Outside `main.go` and Tests

`panic()` is restricted to fatal startup misconfiguration (e.g., `internal/api/router.go` guarding required deps, `internal/auth/jwt.go` on `crypto/rand` failure) and `main.go` / test files. Return early on errors. Keep symbols unexported unless cross-package access is required. Sources: code-patterns, CLAUDE.md.

### snake_case Go Filenames

All Go source files use snake_case (lowercase with underscores) — no camelCase or PascalCase Go filenames. Examples: `internal/api/capability_handlers.go`, `internal/store/user_store_lockout.go`. Source: code-patterns (73 of 80 non-test .go files).

### Three-Group Import Ordering

Imports are separated by blank lines into three groups: stdlib, third-party, internal (github.com/PawelHaracz/agentlens/...). Example:

```go
import (
    "context"
    "fmt"

    "gorm.io/gorm"

    "github.com/PawelHaracz/agentlens/internal/model"
)
```

Sources: code-patterns.

### errcheck Enforced — Do Not Ignore Error Returns

All error returns (os.Stdout.WriteString, sqlDB.Close, json.Encoder.Encode, require.NoError in tests) must be assigned to `_` or handled. CI fails on errcheck violations. Source: PR reviews (#4, #15, #9).

### Sort Go Map Keys Before Serializing or Building Derived Identifiers

Go map iteration order is non-deterministic. Any code that flows map data into stored capabilities, API responses, UI rendering, or unique-constraint-sensitive derived names must sort keys first (e.g., sorted scheme names, sorted flow types). Otherwise UI order, tests, and DB UNIQUE-constraint hits become flaky. Source: PR reviews (#18 — 4 instances in one PR).

### Exported Identifier Doc Comments

All exported types, methods, and constructors carry a doc comment starting with the identifier name per Go convention. Example: `// NewHandler creates a new Handler with the given kernel.` Sources: code-patterns (78% of sampled files), CONTRIBUTING.md.

### chi Handler Signature and Response Helpers

HTTP handlers are methods on `*Handler` (or constructors returning `http.HandlerFunc`) with `(w http.ResponseWriter, r *http.Request)`. Responses go through `JSONResponse` / `ErrorResponse` helpers — not direct `w.Write`. Router composes middleware: Recovery, Logger, CORS, RequestID, otelhttp. Example: `func (h *Handler) ListCatalog(w http.ResponseWriter, r *http.Request)`. Source: code-patterns (40 occurrences across 14 files).

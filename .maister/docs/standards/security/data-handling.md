# Security: Data Handling

Hardening rules around secrets, database access, error messages, and CORS. These are the non-negotiable security rules from CLAUDE.md.

### Never Log Secrets

Never log passwords, tokens, or any secret material via slog, error messages, or comments. Sources: CLAUDE.md, AGENTS.md.

### Password Hash Field Annotations

The `password_hash` field must always be tagged `json:"-"` and `gorm:"type:text"`. Prevents accidental exposure via JSON and pins storage to TEXT (not fixed-size varchar). Applies to any sensitive field — failed-attempt counters, lock timestamps, raw agent card bytes must also use `json:"-"`. Example: ``PasswordHash string `json:"-" gorm:"not null;type:text"` ``. Sources: CLAUDE.md, code-patterns (20 occurrences in internal/model/).

### Parameterized Queries Only

All database access must go through GORM parameterized queries (`tx.First(&user, "id = ?", id)` / `.Where("... = ?", v)`). Raw SQL string interpolation is forbidden. Sources: CLAUDE.md, docs/devops-guide.md, code-patterns.

### Input Validation at the API Boundary

Validate inputs (UUIDs, enums, required fields, format validators such as `kind::name`) at the API boundary. Trim string inputs; reject whitespace-only names. Map expected store errors to stable HTTP codes: duplicate → 409, missing/invalid → 400, not-found → 404. Reserve 500 for unexpected errors. Sources: CLAUDE.md, PR reviews (#1, #17, #23).

### Don't Leak Internal Error Detail

Handlers must return generic messages (e.g., "failed to list capabilities") and log the wrapped error server-side via `slog.InfoContext(r.Context(), ...)`. Don't echo SQL or raw network errors to clients. Prefer typed sentinel errors (`errors.Is`) over string matching on `err.Error()`. Sources: PR reviews (#15, #17).

### CORS Must Stay Restricted

The current CORS configuration is `*`. Do not widen it further without explicit approval. Track this as tech debt (see roadmap). Source: CLAUDE.md.

# Testing: Go

Go tests use table-driven subtests with testify and in-memory SQLite. No external database is required.

### Table-Driven Subtests with `t.Run` and testify

Use table-driven tests with `t.Run` subtests. Assertions use `testify/require` for fatal checks and `testify/assert` for non-fatal. Place `_test.go` files alongside the code they test. Sources: code-patterns (15+ test files), CLAUDE.md, docs/developer-guide.md.

```go
func TestSomething(t *testing.T) {
    cases := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid", "ok", false},
        {"empty", "", true},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            err := DoSomething(tc.input)
            if tc.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.NotEmpty(t, tc.input)
        })
    }
}
```

### In-Memory SQLite for Store Tests

Store and plugin tests must use in-memory SQLite via `store.NewSQLiteStore(":memory:")` or `db.OpenMemory()`. No external DB required. Sources: code-patterns (31 occurrences across 15 files), CLAUDE.md.

```go
s, err := store.NewSQLiteStore(":memory:")
require.NoError(t, err)
```

### API Handler Test Coverage

API handler tests must exercise status codes, response shape, and auth enforcement. Import-handler tests must cover both authenticated and unauthenticated paths. Source: CLAUDE.md, PR review (#7).

### Register `t.Cleanup` to Close In-Memory DB

Tests that open `db.OpenMemory()` or `store.NewSQLiteStore(":memory:")` must register `t.Cleanup(func() { ... })` to close the DB handle and drop resources. Follow the pattern in `internal/db/migrate_test.go` `testDB` helper. Otherwise FDs leak across the suite. Source: PR reviews (#23, #16).

```go
d, err := db.OpenMemory()
require.NoError(t, err)
t.Cleanup(func() { _ = d.Close() })
```

### Account Lockout — Use Separate Accounts, Don't Bypass

Tests that exercise login must not bypass the 5-fail / 15-minute lockout logic. Use separate accounts per test case instead. Source: CLAUDE.md.

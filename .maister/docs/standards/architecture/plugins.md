# Architecture: Plugin System

Plugins extend the kernel via a uniform contract. They live under `plugins/` and may only depend on `kernel` + `foundation`.

### Plugin Struct Suffix Rule

Any struct implementing the `Plugin` interface must have a type name ending with `Plugin`. In per-plugin subpackages, the canonical name is simply `Plugin` — package namespacing handles disambiguation (e.g. `a2a.Plugin`, `mcp.Plugin`), which satisfies the suffix rule. Enforced by arch-go `namingRules.interfaceImplementationNamingRule`. All 10 current plugin structs comply.

Example (`plugins/parsers/a2a/a2a.go`):

```go
type Plugin struct {
    initialized bool
    metrics     *telemetry.ParserMetrics
}
```

Sources: `arch-go.yml`, code patterns, `CLAUDE.md`.

### Plugin Lifecycle Contract

All plugins implement the uniform lifecycle:

```
Register → InitAll → StartAll → [running] → StopAll
```

- **Register** — adds the plugin to the manager; no side effects.
- **Init** — configures the plugin. Returning `ErrLicenseRequired` causes the plugin to be silently skipped (it is not started or stopped).
- **Start** — begins active work (goroutines, watchers, timers).
- **Stop** — ends active work cleanly.

Sources: `CLAUDE.md`, ADR-003.

### Kernel Isolation

Plugins depend on the kernel **interface**, never on concrete kernel internals. The kernel does not import plugins — plugins register themselves into the kernel from the composition root (`cmd/**`). This preserves the unidirectional dependency and keeps the plugin boundary testable.

Source: ADR-003.

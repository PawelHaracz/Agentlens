# Architecture: Layering

AgentLens enforces strict layer boundaries via arch-go at 100% compliance. Layers align with microkernel + plugins architecture.

### Strict Layer Boundaries (arch-go enforced at 100%)

Packages must obey the following dependency rules:

- **foundation** (`model`, `config`, `service`) — no internal deps
- **infrastructure** (`store`, `auth`, `db`, `telemetry`) — foundation only
- **core** (`kernel`, `discovery`, `health`, `server`) — foundation + infrastructure
- **api** — core + infrastructure; NEVER `plugins/**` or `cmd/**`
- **plugins** (`plugins/**`) — `kernel` + foundation only; NEVER `api`, `auth`, `server`, or `cmd`
- **cmd** (entrypoint) — composition root; may import anything

Enforced by `arch-go.yml`:

```yaml
threshold:
  compliance: 100
  coverage: 80
```

Violations fail CI. Sources: `arch-go.yml`, `CLAUDE.md`, ADR-003.

### Function Complexity Limits

Functions in `internal/**` and `plugins/**` must respect:

- Max **5 parameters**
- Max **3 return values**
- Max **80 lines** per function
- Max **10 public functions** per file

Enforced by arch-go `functionsRules`. Sources: `arch-go.yml`, `CLAUDE.md`.

### No Interfaces in the `config` Package

The `internal/config` package must contain **no Go interfaces** — pure configuration structs only. Configuration is data; polymorphism belongs elsewhere.

Enforced by arch-go:

```yaml
contentsRules:
  - package: "**/internal/config"
    shouldNotContainInterfaces: true
```

Sources: `arch-go.yml`, `CLAUDE.md`.

### Add `namingRules` Once Pattern Appears 3+ Times

When a naming pattern emerges across three or more instances, codify it in `arch-go.yml` under `namingRules` so it is enforced going forward. This keeps conventions discoverable and prevents drift.

Source: `CLAUDE.md`.

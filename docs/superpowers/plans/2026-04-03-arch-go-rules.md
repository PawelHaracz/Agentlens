# arch-go Architectural Rules Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce microkernel layer boundaries and code quality via arch-go, integrated into the Makefile pipeline.

**Architecture:** Standalone `arch-go.yml` at repo root defines dependency, contents, functions, and naming rules. A new `make arch-test` target runs the CLI. Added to `make all`.

**Tech Stack:** arch-go v2 CLI, YAML config, Make

---

### Task 1: Install arch-go and create `arch-go.yml`

**Files:**
- Create: `arch-go.yml`

- [ ] **Step 1: Install arch-go**

Run:
```bash
go install -v github.com/arch-go/arch-go/v2@latest
```

Verify:
```bash
arch-go --version
```

- [ ] **Step 2: Create `arch-go.yml` with all rules**

Create file `arch-go.yml` at repo root with this exact content:

```yaml
version: 1

threshold:
  compliance: 100
  coverage: 80

dependenciesRules:
  # Foundation layer — no internal dependencies allowed
  - package: "**.internal.model"
    shouldNotDependsOn:
      internal:
        - "**.internal.api"
        - "**.internal.auth"
        - "**.internal.config"
        - "**.internal.db"
        - "**.internal.discovery"
        - "**.internal.health"
        - "**.internal.kernel"
        - "**.internal.server"
        - "**.internal.service"
        - "**.internal.store"
        - "**.plugins.**"
        - "**.cmd.**"

  - package: "**.internal.config"
    shouldNotDependsOn:
      internal:
        - "**.internal.api"
        - "**.internal.auth"
        - "**.internal.db"
        - "**.internal.discovery"
        - "**.internal.health"
        - "**.internal.kernel"
        - "**.internal.model"
        - "**.internal.server"
        - "**.internal.service"
        - "**.internal.store"
        - "**.plugins.**"
        - "**.cmd.**"

  - package: "**.internal.service"
    shouldNotDependsOn:
      internal:
        - "**.internal.api"
        - "**.internal.auth"
        - "**.internal.config"
        - "**.internal.db"
        - "**.internal.discovery"
        - "**.internal.health"
        - "**.internal.kernel"
        - "**.internal.model"
        - "**.internal.server"
        - "**.internal.store"
        - "**.plugins.**"
        - "**.cmd.**"

  # Core layer — kernel must not depend on api, auth, server, or plugins
  - package: "**.internal.kernel"
    shouldNotDependsOn:
      internal:
        - "**.internal.api"
        - "**.internal.auth"
        - "**.internal.server"
        - "**.internal.service"
        - "**.plugins.**"
        - "**.cmd.**"

  # Infrastructure — store must not depend on upper layers
  - package: "**.internal.store"
    shouldNotDependsOn:
      internal:
        - "**.internal.api"
        - "**.internal.auth"
        - "**.internal.kernel"
        - "**.internal.server"
        - "**.internal.service"
        - "**.plugins.**"
        - "**.cmd.**"

  # Plugins — may only depend on kernel + foundation, never on api/auth/server/cmd
  - package: "**.plugins.**"
    shouldNotDependsOn:
      internal:
        - "**.internal.api"
        - "**.internal.auth"
        - "**.internal.server"
        - "**.internal.service"
        - "**.cmd.**"

  # API layer — must not import plugins or cmd directly
  - package: "**.internal.api"
    shouldNotDependsOn:
      internal:
        - "**.cmd.**"
        - "**.plugins.**"

contentsRules:
  # model is pure domain types — no interfaces
  - package: "**.internal.model"
    shouldNotContainInterfaces: true

  # config is pure config structs — no interfaces
  - package: "**.internal.config"
    shouldNotContainInterfaces: true

functionsRules:
  - package: "**.internal.**"
    maxParameters: 5
    maxReturnValues: 3
    maxLines: 80
    maxPublicFunctionPerFile: 10

  - package: "**.plugins.**"
    maxParameters: 5
    maxReturnValues: 3
    maxLines: 80
    maxPublicFunctionPerFile: 10

namingRules:
  - package: "**.plugins.**"
    interfaceImplementationNamingRule:
      structsThatImplement:
        internal: "*Plugin"
      shouldHaveSimpleNameEndingWith: "Plugin"
```

- [ ] **Step 3: Run arch-go to verify rules pass**

Run:
```bash
arch-go
```

Expected: all rules pass (compliance 100%). If any functions rules fail (e.g. a function exceeds 80 lines), note the violations — they will be fixed in Task 2.

- [ ] **Step 4: Commit**

```bash
git add arch-go.yml
git commit -m "feat: add arch-go.yml with architectural rules"
```

---

### Task 2: Fix any arch-go violations

**Files:**
- Modify: whichever files arch-go reports as violations

- [ ] **Step 1: Run arch-go and capture violations**

Run:
```bash
arch-go 2>&1 | tee /tmp/arch-go-report.txt
```

Read the report. The most likely violations are functions exceeding `maxLines: 80`. Common candidates:
- `plugins/parsers/a2a/a2a.go` — `Parse` method (~100 lines)
- `plugins/parsers/a2a/validation.go` — `ValidateCard` function (~100 lines)
- `internal/api/handlers.go` — `ListCatalog` with many query params

- [ ] **Step 2: Refactor each violation**

For each function exceeding 80 lines, extract helper functions. For example, if `Parse` in `a2a.go` is too long, extract `buildSkills(card)`, `buildProvider(card)`, `buildTypedMeta(card)` helpers.

Rules for refactoring:
- Extract private helper functions within the same file
- Do NOT change public API signatures
- Run `make test` after each refactor to ensure nothing breaks

- [ ] **Step 3: Re-run arch-go to verify all rules pass**

Run:
```bash
arch-go
```

Expected: `compliance: 100%`, all rules PASS.

- [ ] **Step 4: Run full test suite**

Run:
```bash
make test
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: fix arch-go violations (function length)"
```

---

### Task 3: Add `make arch-test` target and update `make all`

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add `arch-test` target to Makefile**

Add after the `tools` target (around line 88), before the Frontend targets section:

```makefile
## arch-test: Run architecture rules validation (arch-go)
arch-test:
	arch-go
```

- [ ] **Step 2: Add `arch-test` to `.PHONY`**

In the `.PHONY` declaration (line 15), add `arch-test`:

```makefile
.PHONY: all check build test lint format run clean help \
        test-coverage test-race vet \
        web-install web-build web-lint web-test \
        e2e-install e2e-test \
        helm-lint docker-build docker-scan \
        deps tools arch-test
```

- [ ] **Step 3: Update `make all` to include `arch-test`**

Change line 29 from:
```makefile
all: format lint test build
```
to:
```makefile
all: format lint test arch-test build
```

- [ ] **Step 4: Verify `make arch-test` works**

Run:
```bash
make arch-test
```

Expected: arch-go runs and passes.

- [ ] **Step 5: Verify `make all` works**

Run:
```bash
make all
```

Expected: format → lint → test → arch-test → build all succeed.

- [ ] **Step 6: Commit**

```bash
git add Makefile
git commit -m "build: add make arch-test target and include in make all"
```

---

### Task 4: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update Commands section**

In CLAUDE.md, find the Commands code block (around line 16-30). Add `make arch-test` and update `make all` description:

Change:
```bash
make all            # format → lint → test → build
```
to:
```bash
make arch-test      # Run arch-go architecture rules validation
make all            # format → lint → test → arch-test → build
```

- [ ] **Step 2: Add Architecture Rules section**

After the "Go Conventions" section, add a new section:

```markdown
## Architecture Rules (arch-go)

The project uses [arch-go](https://github.com/arch-go/arch-go) to enforce microkernel layer boundaries at CI time. Rules are defined in `arch-go.yml` at the repo root.

### Layer boundaries enforced

- **Foundation** (`model`, `config`, `service`) — no internal dependencies
- **Infrastructure** (`store`, `auth`) — depends on foundation only
- **Core** (`kernel`, `discovery`) — depends on foundation + infrastructure
- **API** (`api`) — depends on core + infrastructure, never plugins or cmd
- **Plugins** (`plugins/**`) — depend on kernel + foundation, never api/auth/server/cmd
- **Entrypoint** (`cmd/**`) — composition root, may import anything

### Code quality rules

- Max 5 function parameters, 3 return values, 80 lines per function, 10 public functions per file
- `model` and `config` packages must not contain interfaces
- Plugin structs implementing `Plugin` interface must end with `Plugin`

### Naming rules expansion policy

When a new naming convention emerges in the codebase (e.g. all stores end with `Store`, all handlers end with `Handler`), add a corresponding `namingRules` entry to `arch-go.yml` to enforce it. Keep naming rules minimal and only add them when a pattern is established across at least 3 instances.

### Running

```bash
make arch-test      # Run architecture validation
arch-go --html      # Generate HTML report in .arch-go/
```
```

- [ ] **Step 3: Add `.arch-go/` to `.gitignore`**

Check if `.gitignore` exists and add `.arch-go/` (arch-go HTML/JSON report output directory):

```bash
echo "" >> .gitignore
echo "# arch-go reports" >> .gitignore
echo ".arch-go/" >> .gitignore
```

- [ ] **Step 4: Verify everything still works**

Run:
```bash
make all
```

Expected: all steps pass.

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md .gitignore
git commit -m "docs: document arch-go rules and naming expansion policy in CLAUDE.md"
```

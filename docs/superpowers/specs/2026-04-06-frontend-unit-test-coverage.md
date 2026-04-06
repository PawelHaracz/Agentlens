# Task: Add comprehensive frontend unit test coverage and enforce it in CI

## Background

The AgentLens frontend (`web/`) is a React 18 + TypeScript + Vite app embedded into the Go binary via `web/embed.go`. Vitest + Testing Library is already wired up (`web/vitest.config.ts`, `src/test-setup.ts`, `make web-test`), and CI already runs `make web-test` in the `test-frontend` job (`.github/workflows/ci.yml:70-84`).

The problem: only **two** test files exist today (`web/src/api.test.ts`, `web/src/components/RegisterAgentDialog.test.tsx`). The rest of the frontend — components, pages, contexts, hooks, utilities — has zero coverage. Refactors break the UI silently and we only catch it in Playwright E2E, which is slow and runs late.

We need real unit-test coverage of the frontend, and we need CI to **enforce** a minimum coverage bar so the bar can't slip back down.

This is a documentation-and-test PR plus a small CI tweak. **Do not refactor production components** to make them easier to test unless a component is genuinely untestable as written; in that case, prefer dependency-injection (e.g. accept fetcher as a prop) over rewriting logic.

## Authoritative sources of truth

1. `web/src/**` — every `.tsx`/`.ts` file under here is in scope. Read each one before testing it.
2. `web/vitest.config.ts` — current test runner config. You will extend this.
3. `web/package.json` — testing libs already installed (`@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event`, `jsdom`, `vitest`).
4. `web/src/api.test.ts` and `web/src/components/RegisterAgentDialog.test.tsx` — match this style for the new tests (RTL queries by role/label, `userEvent` for interaction, `fetch` mocked via `vi.stubGlobal`).
5. `.github/workflows/ci.yml` — CI definition you'll modify.
6. `Makefile` — `make web-test`, `make web-lint`, `make web-install` already exist; add a `make web-test-coverage` target.

## Scope

### 1. Test the entire `web/src` tree

For every source file, write tests that exercise the **observable behavior** the user (or another component) depends on. Co-locate tests next to the source: `Foo.tsx` → `Foo.test.tsx`.

Concrete coverage targets — read each file first, then write tests:

**`web/src/components/`**

| File | What to test |
|---|---|
| `CatalogList.tsx` | Renders rows from props/data, empty state, loading state, click on row navigates to detail, sort/column behavior if present. |
| `EntryDetail.tsx` | Renders all sections (header, capabilities tabs, raw JSON), tab switching, empty capability state, delete action calls handler with correct id, error state. |
| `RegisterAgentDialog.tsx` | (already partially tested — extend) all three register flows: import-from-URL happy path, import-from-URL private/loopback rejection (mock API 400), paste-JSON happy path, paste-JSON validation error, dialog open/close, success callback fires with new entry, error toasts. |
| `CardPreview.tsx` | Renders parsed card fields, handles missing optional fields, protocol-specific rendering. |
| `Layout.tsx` | Renders nav links, user dropdown opens, logout calls auth context, active route highlighting. |
| `ProtectedRoute.tsx` | Redirects to `/login` when unauthenticated, renders children when authenticated, respects required permission if used. |
| `ProtocolBadge.tsx` | Renders the right label and class for each `Protocol` value, falls back gracefully on unknown. |
| `StatusBadge.tsx` | Same idea — every `Status` value, fallback. |
| `StatsBar.tsx` | Renders stat values from props, handles zero/empty. |
| `SearchBar.tsx` | Calls `onChange`/`onSearch` with debouncing if present, clears on empty, controlled vs uncontrolled. |

**`web/src/pages/`**

Cover every page component. For each: render under a `MemoryRouter` with seeded auth context, mock the relevant API calls via `vi.stubGlobal('fetch', …)`, assert headings/key UI elements appear, assert error states render on API failure. Pages typically include catalog list, entry detail, login, settings (general/account/users/roles).

**`web/src/contexts/`**

For every context (e.g. `AuthContext`, any toast/notification context):

- Provider initial state matches expectations.
- State transitions (login → token stored, logout → token cleared).
- Consumer hook returns the expected shape.
- Error if used outside provider (if guarded).

**`web/src/lib/`**

Pure functions — easy wins, aim for 100% line coverage here. Test all branches and edge cases (empty input, null, malformed).

**`web/src/ui/`** (shadcn-style primitives)

Skip deep tests of upstream Radix wrappers. Only test ones we have meaningfully customized. Add a brief smoke test (renders, accepts ref, forwards `className`) for each customized primitive.

**`web/src/api.ts`**

Already has `api.test.ts`. Audit it: every exported function in `api.ts` must have at least one happy-path test and one error-path test. Add what's missing. Mock `fetch` consistently with the existing pattern.

**`web/src/App.tsx`, `web/src/main.tsx`**

`main.tsx` is a tiny bootstrap — skip.
`App.tsx` — test that routes mount the right page components and that `ProtectedRoute` wraps protected routes.

### 2. Coverage configuration

Extend `web/vitest.config.ts` with a `test.coverage` block using the v8 provider:

```ts
test: {
  environment: 'jsdom',
  globals: true,
  setupFiles: ['./src/test-setup.ts'],
  coverage: {
    provider: 'v8',
    reporter: ['text', 'text-summary', 'html', 'lcov'],
    reportsDirectory: './coverage',
    include: ['src/**/*.{ts,tsx}'],
    exclude: [
      'src/main.tsx',
      'src/test-setup.ts',
      'src/**/*.d.ts',
      'src/**/*.test.{ts,tsx}',
      'src/ui/**',         // shadcn primitives — covered by smoke tests only
      'src/types.ts',      // pure type declarations
    ],
    thresholds: {
      lines: 80,
      functions: 80,
      branches: 75,
      statements: 80,
    },
  },
},
```

The thresholds are the **gate** — Vitest will exit non-zero if any of them are not met, which is exactly what we want for CI.

Add the v8 coverage provider to `devDependencies`:

```bash
cd web && bun add -D @vitest/coverage-v8
```

### 3. Makefile target

Add to `Makefile`:

```make
.PHONY: web-test-coverage
web-test-coverage: ## Run frontend unit tests with coverage and threshold enforcement
	cd web && bun run test -- --coverage
```

(Or, equivalently, add a `test:coverage` script to `web/package.json` and call it. Pick one and stay consistent — prefer the Makefile target for symmetry with `make test-coverage` on the Go side.)

### 4. CI integration

Edit `.github/workflows/ci.yml`, `test-frontend` job (lines 70-84). Replace `make web-test` with `make web-test-coverage` and upload the report:

```yaml
  test-frontend:
    name: Test Frontend
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: oven-sh/setup-bun@v2
        with:
          bun-version-file: ".bun-version"

      - name: Install frontend dependencies
        run: make web-install

      - name: Run frontend unit tests with coverage
        run: make web-test-coverage

      - name: Upload frontend coverage artifact
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: web-coverage-report
          path: web/coverage/
          retention-days: 14
```

The `if: always()` ensures the report is uploaded even when tests fail, so reviewers can inspect what dropped below threshold.

Do **not** add the report to the `build` job — `test-frontend` is already a `needs:` of `build` (ci.yml:89), so a coverage failure already blocks the build.

### 5. Test conventions

Match the existing style — do not introduce new patterns:

- File naming: `Foo.tsx` → `Foo.test.tsx`, co-located.
- Use `@testing-library/react` queries by accessibility role first (`getByRole`, `getByLabelText`), text content second, `data-testid` only as a last resort.
- Use `@testing-library/user-event` (already installed) for clicks/typing — never `fireEvent` directly.
- Mock `fetch` with `vi.stubGlobal('fetch', vi.fn().mockResolvedValue(...))`. Reset between tests with `beforeEach(() => vi.unstubAllGlobals())`.
- Each test wraps components that need routing in `<MemoryRouter>` and components that need auth in the real `AuthContext.Provider` with a seeded value — don't mock the context itself.
- One assertion per logical behavior. No assertion-free tests.
- No snapshot tests — they rot. Assert specific text/roles instead.
- Don't test implementation details (state shape, hook internals). Test what the user sees.
- Keep individual test files under ~300 lines. Split by concern if a component has many flows.

## Verification before opening the PR

Run all of these and make sure they're green:

1. `cd web && bun install` — coverage provider installed.
2. `make web-lint` — TypeScript clean.
3. `make web-test-coverage` — runs locally, **all thresholds met**, exits 0.
4. `make web-build` — production build still succeeds.
5. `make build` — Go binary still embeds the frontend correctly.
6. `make test` — Go unit tests untouched and still pass.
7. `make e2e-test` — Playwright suite still passes (you should not have changed any production component behavior).
8. Open the generated `web/coverage/index.html` locally and spot-check that no critical file is at 0% coverage.

## Pull request

- Title: `test(web): add unit test coverage and enforce 80% threshold in CI`
- Body must include:
  - Bullet list of which files/components got new tests.
  - The coverage summary line (`% Stmts | % Branch | % Funcs | % Lines`) from the local run, before and after.
  - The exact `make` commands run for verification, with status.
  - Any files explicitly excluded from coverage with a one-line justification.
  - Any production component that needed a small testability tweak (e.g. extracted a prop), with rationale.
- Do not commit `web/coverage/` — add it to `.gitignore` if not already ignored.
- Do not commit anything under `web/node_modules/`.

## Things NOT to do

- Do not lower the coverage thresholds to make them pass. If you can't reach 80%/75%, surface it in the PR description and explain why.
- Do not add tests that just call functions for line coverage without assertions ("coverage theatre").
- Do not refactor production code to "make it testable" beyond extracting a prop or splitting a file. Any larger refactor is out of scope and needs its own PR.
- Do not introduce a different test runner, assertion library, or DOM environment. Vitest + RTL + jsdom is the standard.
- Do not test the shadcn `ui/` primitives deeply — they are upstream code.
- Do not modify `internal/**` or any Go code.
- Do not modify `e2e/**` Playwright tests.
- Do not change the `build` or `lint` jobs in `ci.yml` — only `test-frontend`.

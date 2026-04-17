# Testing: Frontend (Vitest)

Frontend tests use Vitest + Testing Library in jsdom. Coverage is enforced in CI.

### Vitest Coverage Thresholds 80/80/75/80

Frontend unit-test coverage must meet: lines 80%, functions 80%, branches 75%, statements 80%. Provider: v8. Reporters: text, text-summary, html, lcov. Excluded: `src/main.tsx`, `src/test-setup.ts`, `src/**/*.d.ts`, `src/**/*.test.{ts,tsx}`, `src/components/ui/**`, `src/types.ts`. Enforced in CI via `make web-test-coverage`. Source: web/vitest.config.ts, ci.yml.

### jsdom + Testing Library Setup

Tests run in `jsdom` with `globals: true` and a shared setup at `./src/test-setup.ts`. Stack: `@testing-library/react` 16, `@testing-library/jest-dom` 6, `@testing-library/user-event` 14. Source: web/vitest.config.ts.

### Co-Located Tests with `.test.tsx` Suffix

Every component/hook/context is paired with a sibling `<Name>.test.tsx` (or `.test.ts`) file in the same directory. Example: `CardPreview.tsx` + `CardPreview.test.tsx`. Source: code-patterns.

### Restore Global Mocks With `vi.spyOn` + `afterEach`

When stubbing globals (`navigator.clipboard`, `window.confirm`), use `vi.spyOn(...).mockReturnValue(...)` and restore in `afterEach`, not direct assignment. Properties may also need `configurable: true` to be respyable. Source: PR reviews (#9, #18).

```ts
let confirmSpy: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
})

afterEach(() => {
  confirmSpy.mockRestore()
})
```

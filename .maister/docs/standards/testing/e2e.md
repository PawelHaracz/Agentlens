# Testing: End-to-End (Playwright)

E2E tests run against a real SQLite-backed agentlens binary. Playwright runs serially because tests share admin state.

### Playwright Serial Single-Worker Run (Chromium)

`e2e/playwright.config.ts`: `fullyParallel: false`, `workers: 1`, test timeout 60s, expect timeout 10s. Chromium-only project. On CI: `forbidOnly: true`, `retries: 1`, `reporter: 'github'`. Locally: `reporter: 'list'`, no retries. Trace on first retry, screenshots only on failure. Base URL: `http://localhost:${AGENTLENS_PORT ?? 18080}`. Source: e2e/playwright.config.ts.

### Real SQLite Backend + Masked Admin Password

`e2e.yml` workflow starts a real agentlens binary (built with CGO_ENABLED=1) on port 18080 with an ephemeral `DATA_DIR`, waits for `/healthz`, and masks the bootstrap admin password via `echo "::add-mask::$ADMIN_PW"` before running `bunx playwright test`. 20-minute CI timeout. Source: .github/workflows/e2e.yml.

### Reuse Shared Playwright Helpers

All E2E specs import `loginViaUI`, `loginViaAPI`, `authHeader`, `BASE`, `ADMIN_USER`, `adminPassword` from `./helpers` (e2e/tests/helpers.ts). Do not reimplement login flows inline. Sources: code-patterns, CLAUDE.md.

```ts
import { loginViaUI, loginViaAPI, authHeader, BASE, ADMIN_USER, adminPassword } from './helpers'

test('admin can see dashboard', async ({ page }) => {
  await loginViaUI(page)
  // ...
})
```

### `data-testid` Screenshots for UI Changes

UI changes require screenshots via `page.screenshot()` using `data-testid` selectors, saved under `docs/images/`. Images are referenced from `docs/end-user-guide.md`. Source: CLAUDE.md, AGENTS.md.

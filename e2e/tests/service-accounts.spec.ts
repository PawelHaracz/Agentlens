/**
 * E2E screenshot test for the Service Accounts and Pending Identities admin pages.
 * Captures docs/images/ screenshots per the PR checklist requirement.
 *
 * Run standalone (with a server already up on AGENTLENS_PORT):
 *   AGENTLENS_PORT=18085 AGENTLENS_ADMIN_PASSWORD=... bun run test -- service-accounts.spec.ts
 *
 * Screenshots saved to docs/images/:
 *   service-accounts-list.png
 *   service-accounts-create.png
 *   service-accounts-secret.png
 *   pending-identities.png
 */
import { test, expect } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { loginViaUI, loginViaAPI, authHeader, BASE } from './helpers';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const DOCS_IMAGES = path.resolve(__dirname, '../../docs/images');

// ── helpers ────────────────────────────────────────────────────────────────

async function createServiceAccount(
  request: APIRequestContext,
  token: string,
  name: string,
): Promise<{ partyId: string; secret: string }> {
  const res = await request.post(`${BASE}/api/v1/service-accounts`, {
    headers: { ...authHeader(token), 'Content-Type': 'application/json' },
    data: { name },
  });
  expect(res.ok(), `createSA failed: ${res.status()}`).toBeTruthy();
  const body = await res.json();
  return { partyId: body.party.id, secret: body.secret };
}

async function createPendingIdentity(
  request: APIRequestContext,
  token: string,
  sub: string,
): Promise<string> {
  // Upsert via the external-identities pending endpoint (used in federation flow).
  // In tests we seed directly via the store; here we simulate via the store-backed
  // route which only exists if the server has federation enabled.
  // Fallback: seed via direct SQL through the test helper is not available in E2E.
  // For screenshot purposes, create a pending identity via the user-external-identity store.
  // Since there is no dedicated "seed" route, skip seeding — page shows empty state.
  void token; void sub;
  return '';
}

// ── tests ──────────────────────────────────────────────────────────────────

test.describe('Service Accounts admin screenshots', () => {
  test('service-accounts-list: two accounts visible', async ({ page, request }) => {
    const token = await loginViaAPI(request);

    // Seed two service accounts.
    await createServiceAccount(request, token, 'production-llm-app');
    await createServiceAccount(request, token, 'dev-chatbot');

    await loginViaUI(page);
    await page.goto('/admin/service-accounts');
    await page.waitForSelector('[data-testid="sa-table"]');
    await expect(page.getByText('production-llm-app').first()).toBeVisible();
    await expect(page.getByText('dev-chatbot').first()).toBeVisible();

    await page.screenshot({
      path: path.join(DOCS_IMAGES, 'service-accounts-list.png'),
      fullPage: false,
    });
  });

  test('service-accounts-create: dialog open', async ({ page, request }) => {
    const token = await loginViaAPI(request);
    await createServiceAccount(request, token, 'existing-sa');

    await loginViaUI(page);
    await page.goto('/admin/service-accounts');
    await page.waitForSelector('[data-testid="create-sa-btn"]');

    await page.click('[data-testid="create-sa-btn"]');
    await expect(page.getByTestId('create-sa-dialog')).toBeVisible();

    await page.screenshot({
      path: path.join(DOCS_IMAGES, 'service-accounts-create.png'),
      fullPage: false,
    });
  });

  test('service-accounts-secret: one-time secret banner after create', async ({ page, request }) => {
    const token = await loginViaAPI(request);
    await createServiceAccount(request, token, 'background-sa');

    await loginViaUI(page);
    await page.goto('/admin/service-accounts');
    await page.waitForSelector('[data-testid="create-sa-btn"]');

    // Open create dialog and submit.
    await page.click('[data-testid="create-sa-btn"]');
    await page.fill('[data-testid="sa-name-input"]', 'analytics-service');
    await page.getByRole('button', { name: 'Create' }).click();

    // Wait for the one-time secret banner.
    await expect(page.getByTestId('one-time-secret-display')).toBeVisible({ timeout: 10_000 });

    await page.screenshot({
      path: path.join(DOCS_IMAGES, 'service-accounts-secret.png'),
      fullPage: false,
    });
  });
});

test.describe('Pending Identities admin screenshots', () => {
  test('pending-identities: empty state page', async ({ page, request }) => {
    void request;
    await loginViaUI(page);
    await page.goto('/admin/external-identities');
    await page.waitForSelector('[data-testid="pending-identities-table"]');

    // Empty state is also useful to document — shows "No pending identities".
    await page.screenshot({
      path: path.join(DOCS_IMAGES, 'pending-identities.png'),
      fullPage: false,
    });
  });
});

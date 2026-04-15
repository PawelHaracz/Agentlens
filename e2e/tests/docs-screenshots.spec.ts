/**
 * docs-screenshots.spec.ts
 *
 * Screenshot GENERATOR for docs/end-user-guide.md.
 * This is NOT an assertion test — it walks through the UI and saves
 * screenshots to docs/images/ for use in the end-user documentation.
 *
 * Run via:  make docs-screenshots
 * Which executes: ./e2e/run-e2e.sh tests/docs-screenshots.spec.ts
 *
 * The script seeds deterministic data, captures screenshots, and then
 * cleans up the seeded entries so that it does not affect other test runs.
 */

import { test } from '@playwright/test';
import path from 'path';
import { fileURLToPath } from 'url';
import {
  loginViaUI,
  loginViaAPI,
  authHeader,
  deleteCatalogEntry,
  createUser,
  BASE,
} from './helpers';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const DOCS_IMAGES = path.resolve(__dirname, '../../docs/images');

const VIEWPORT = { width: 1680, height: 1050 };

/** Timeout for error/validation elements to become visible. */
const ERROR_DISPLAY_TIMEOUT_MS = 10_000;

/** Timeout for async operations (validation, import). */
const ASYNC_OP_TIMEOUT_MS = 15_000;

/** The public URL used in the "filled" import screenshot. */
const DEMO_IMPORT_URL = 'https://demo-translator.example.com/.well-known/agent.json';

// ────────── seed data ──────────
/** Suppress and log errors from best-effort cleanup operations. */
function ignoreCleanupError(label: string) {
  return (err: unknown) => console.warn(`[docs-screenshots] cleanup warning (${label}):`, err);
}

/**
 * Create a catalog entry, or return the existing entry's ID if the endpoint
 * already exists (409). This makes beforeAll idempotent across re-runs.
 */
async function seedOrFind(
  request: import('@playwright/test').APIRequestContext,
  token: string,
  body: Record<string, unknown>,
): Promise<string> {
  const res = await request.post(`${BASE}/api/v1/catalog`, {
    headers: authHeader(token),
    data: body,
  });
  if (res.ok()) {
    const entry = await res.json();
    return entry.id as string;
  }
  if (res.status() === 409) {
    // Entry already exists — scan all entries and match by endpoint (most reliable).
    // Retry a few times in case of a write-then-read race on a shared database.
    const endpoint = body.endpoint as string;
    const displayName = body.display_name as string;
    for (let attempt = 0; attempt < 5; attempt++) {
      if (attempt > 0) {
        await new Promise(resolve => setTimeout(resolve, 500 * attempt));
      }
      const allRes = await request.get(`${BASE}/api/v1/catalog`, {
        headers: authHeader(token),
      });
      if (allRes.ok()) {
        const all: Array<{ id: string; endpoint: string; display_name: string }> =
          await allRes.json();
        const existing =
          all.find(e => e.endpoint === endpoint) ??
          all.find(e => e.display_name === displayName);
        if (existing) return existing.id;
        console.warn(
          `[seedOrFind] attempt ${attempt + 1}: 409 but "${endpoint}" not found in`,
          all.map(e => `${e.display_name}:${e.endpoint}`),
        );
      }
    }
  }
  const body2 = await res.text().catch(() => '(body unreadable)');
  throw new Error(`seedOrFind failed: ${res.status()} ${body2}`);
}

// A2A entry with skills — used for paste-JSON screenshots
const A2A_CARD = JSON.stringify({
  name: 'Demo Translator Agent',
  description: 'Translates text between languages using neural machine translation.',
  version: '1.2.0',
  supportedInterfaces: [{ type: 'a2a', version: '1.0' }],
  skills: [
    { id: 'translate', name: 'Translate', description: 'Translate text from one language to another.' },
    { id: 'detect-language', name: 'Detect Language', description: 'Detect the language of a given text.' },
  ],
});

/**
 * Full A2A card with skills and a valid endpoint URL, used to seed capability data
 * via POST /api/v1/catalog/register (which runs the parser and stores capabilities).
 * Uses a distinct endpoint to avoid conflict with the seedOrFind-seeded a2a entry.
 */
const A2A_CARD_WITH_ENDPOINT = JSON.stringify({
  name: 'Demo Translator Agent',
  description: 'Translates text between languages using neural machine translation.',
  version: '1.2.0',
  url: 'https://demo-translator-cap.example.com',
  provider: {
    organization: 'LangTech Demo',
    url: 'https://langtech-demo.example.com',
  },
  skills: [
    {
      id: 'translate',
      name: 'Translate',
      description: 'Translate text from one language to another.',
      tags: ['translation', 'text'],
    },
    {
      id: 'detect-language',
      name: 'Detect Language',
      description: 'Detect the language of a given text.',
      tags: ['detection', 'nlp'],
    },
  ],
});

/**
 * Register an agent from a raw card JSON via POST /api/v1/catalog/register.
 * Returns the created entry's ID. Uses seedOrFind-style idempotency: if the
 * endpoint already exists (409), look up and return the existing entry's ID.
 */
async function registerFromCard(
  request: import('@playwright/test').APIRequestContext,
  token: string,
  cardJson: string,
): Promise<string> {
  const res = await request.post(`${BASE}/api/v1/catalog/register`, {
    headers: { ...authHeader(token), 'Content-Type': 'application/json' },
    data: cardJson,
  });
  if (res.ok()) {
    const entry = await res.json();
    return entry.id as string;
  }
  if (res.status() === 409) {
    // Entry already exists — look up by endpoint (parsed from card JSON).
    const card = JSON.parse(cardJson) as { url?: string; name?: string };
    const endpoint = card.url ?? '';
    const displayName = card.name ?? '';
    const allRes = await request.get(`${BASE}/api/v1/catalog`, {
      headers: authHeader(token),
    });
    if (allRes.ok()) {
      const all: Array<{ id: string; endpoint: string; display_name: string }> = await allRes.json();
      const existing =
        all.find(e => endpoint && e.endpoint === endpoint) ??
        all.find(e => displayName && e.display_name === displayName);
      if (existing) return existing.id;
    }
  }
  const body = await res.text().catch(() => '(body unreadable)');
  throw new Error(`registerFromCard failed: ${res.status()} ${body}`);
}


test.describe('Documentation Screenshots', () => {
  let token: string;
  let a2aEntryId: string;
  let mcpEntryId: string;
  let a2aCapEntryId: string; // Entry seeded via register (has capabilities)
  let viewerUserId: string;
  let projectId: string; // Project party for catalog-project-filter screenshot
  let demoGroupId: string; // Group party for groups screenshots

  test.beforeAll(async ({ request }) => {
    token = await loginViaAPI(request);

    // Seed A2A entry — if the endpoint already exists from a previous run, reuse it.
    a2aEntryId = await seedOrFind(request, token, {
      display_name: 'Demo Translator Agent',
      description: 'Translates text between languages using neural machine translation.',
      protocol: 'a2a',
      endpoint: 'https://demo-translator.example.com',
      version: '1.2.0',
    });

    // Seed MCP entry — if the endpoint already exists from a previous run, reuse it.
    mcpEntryId = await seedOrFind(request, token, {
      display_name: 'Demo File System MCP',
      description: 'Provides file-system access to MCP-capable AI assistants.',
      protocol: 'mcp',
      endpoint: 'https://demo-fs-mcp.example.com',
      version: '0.8.0',
    });

    // Seed A2A entry WITH capabilities via the register endpoint (full card parse).
    // This is required for capability screenshots — the simple CreateEntry endpoint
    // does not process skills/capabilities from the card.
    a2aCapEntryId = await registerFromCard(request, token, A2A_CARD_WITH_ENDPOINT);

    // Activate the cap entry so it appears in the capabilities list.
    // ListCapabilities only returns active/degraded entries (registered entries are excluded).
    await request.patch(`${BASE}/api/v1/catalog/${a2aCapEntryId}/lifecycle`, {
      headers: authHeader(token),
      data: { state: 'active' },
    });

    // Seed a named project for the project-filter screenshot.
    const projRes = await request.post(`${BASE}/api/v1/projects`, {
      headers: authHeader(token),
      data: { name: 'Docs Demo Project' },
    });
    if (projRes.ok()) {
      const proj = await projRes.json();
      projectId = proj.id as string;
      // Assign the A2A entry to the project.
      await request.post(`${BASE}/api/v1/catalog/${a2aEntryId}/projects`, {
        headers: authHeader(token),
        data: { project_id: projectId },
      });
      // Add admin's Person party as project:owner so the detail screenshot shows a populated members table.
      const partiesRes = await request.get(`${BASE}/api/v1/parties?kind=person`, { headers: authHeader(token) });
      if (partiesRes.ok()) {
        const parties = (await partiesRes.json()) as Array<{ id: string }>;
        if (parties[0] && projectId) {
          await request.post(`${BASE}/api/v1/projects/${projectId}/members`, {
            headers: authHeader(token),
            data: { party_id: parties[0].id, role: 'project:owner' },
          }).catch(() => {});
        }
      }
    }

    // Seed viewer user for settings screenshots
    // Get roles list first
    const rolesRes = await request.get(`${BASE}/api/v1/roles`, {
      headers: authHeader(token),
    });
    const roles = await rolesRes.json();
    const viewerRole = roles.find((r: { name: string }) => r.name === 'viewer');
    if (viewerRole) {
      const viewerUser = await createUser(request, token, {
        username: 'demo_viewer',
        password: 'Viewer@Demo99!',
        display_name: 'Demo Viewer',
        email: 'demo.viewer@example.com',
        role_id: viewerRole.id,
      });
      viewerUserId = viewerUser.id;
    }

    // Seed a demo group with a nested subgroup and the admin person for screenshots.
    const groupRes = await request.post(`${BASE}/api/v1/groups`, {
      headers: authHeader(token),
      data: { name: 'docs-demo-group' },
    });
    if (groupRes.ok()) {
      demoGroupId = (await groupRes.json()).id as string;
      // Nested subgroup
      const subRes = await request.post(`${BASE}/api/v1/groups`, {
        headers: authHeader(token),
        data: { name: 'docs-demo-subgroup' },
      });
      if (subRes.ok()) {
        const subId = (await subRes.json()).id as string;
        await request.post(`${BASE}/api/v1/groups/${demoGroupId}/members`, {
          headers: authHeader(token),
          data: { party_id: subId, role: 'member' },
        }).catch(() => {});
      }
      // Add admin's person party as a member
      const partiesRes = await request.get(`${BASE}/api/v1/parties?kind=person`, {
        headers: authHeader(token),
      });
      if (partiesRes.ok()) {
        const parties = (await partiesRes.json()) as Array<{ id: string; user_id?: string; name: string }>;
        const adminPerson = parties[0];
        if (adminPerson) {
          await request.post(`${BASE}/api/v1/groups/${demoGroupId}/members`, {
            headers: authHeader(token),
            data: { party_id: adminPerson.id, role: 'member' },
          }).catch(() => {});
        }
      }
    }
  });

  test.afterAll(async ({ request }) => {
    // Clean up seeded entries
    if (a2aEntryId) {
      await deleteCatalogEntry(request, token, a2aEntryId).catch(ignoreCleanupError('delete a2a entry'));
    }
    if (mcpEntryId) {
      await deleteCatalogEntry(request, token, mcpEntryId).catch(ignoreCleanupError('delete mcp entry'));
    }
    if (a2aCapEntryId && a2aCapEntryId !== a2aEntryId) {
      await deleteCatalogEntry(request, token, a2aCapEntryId).catch(ignoreCleanupError('delete a2a-cap entry'));
    }
    if (viewerUserId) {
      await request.delete(`${BASE}/api/v1/users/${viewerUserId}`, {
        headers: authHeader(token),
      }).catch(ignoreCleanupError('delete viewer user'));
    }
    if (projectId) {
      await request.delete(`${BASE}/api/v1/projects/${projectId}`, {
        headers: authHeader(token),
      }).catch(ignoreCleanupError('delete docs-demo project'));
    }
    // Delete any docs-demo groups (also removes member relationships via cascade)
    const listRes = await request.get(`${BASE}/api/v1/groups`, { headers: authHeader(token) });
    if (listRes.ok()) {
      const groups = (await listRes.json()) as Array<{ id: string; name: string }>;
      for (const g of groups.filter(g => g.name.startsWith('docs-demo'))) {
        await request.delete(`${BASE}/api/v1/groups/${g.id}`, {
          headers: authHeader(token),
        }).catch(ignoreCleanupError(`delete ${g.name}`));
      }
    }
  });

  // ───────── Login page screenshots ─────────

  test('login-page', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/login-page.png`, fullPage: false });
  });

  test('login-error', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await page.goto('/login');
    await page.getByLabel('Username').fill('admin');
    await page.getByLabel('Password').fill('wrongpassword!!');
    await page.getByRole('button', { name: 'Sign in' }).click();
    // Wait for error message
    await page.waitForSelector('.text-destructive', { timeout: ERROR_DISPLAY_TIMEOUT_MS });
    await page.screenshot({ path: `${DOCS_IMAGES}/login-error.png`, fullPage: false });
  });

  // ───────── Dashboard / catalog overview ─────────

  test('dashboard-overview', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/dashboard-overview.png`, fullPage: true });
  });

  // ───────── Search and filter screenshots ─────────

  test('catalog-search', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    // Type into the unified search box
    await page.getByPlaceholder(/search/i).fill('Translator');
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/catalog-search.png`, fullPage: false });
  });

  test('catalog-unified-search', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    // Focus and type into the unified search box (wide input with placeholder text)
    await page.getByPlaceholder(/search across/i).fill('Translator');
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/catalog-unified-search.png`, fullPage: false });
  });

  test('catalog-filter-protocol', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    // ToggleGroup with type="single" renders ToggleGroupItems as role="radio" (Radix UI).
    await page.getByRole('radio', { name: 'A2A' }).click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/catalog-filter-protocol.png`, fullPage: false });
  });

  test('catalog-protocol-filter', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    // ToggleGroup with type="single" renders ToggleGroupItems as role="radio" (Radix UI).
    await page.getByRole('radio', { name: 'MCP' }).click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/catalog-protocol-filter.png`, fullPage: false });
  });

  // ───────── Agent detail screenshots ─────────

  test('entry-detail-a2a', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    // Navigate to the A2A entry
    await page.goto(`/catalog/${a2aEntryId}`);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/entry-detail-a2a.png`, fullPage: false });
  });

  test('entry-detail-overview-tab', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto(`/catalog/${a2aEntryId}`);
    await page.waitForLoadState('networkidle');
    // Overview tab is selected by default — click it to be explicit
    await page.getByRole('tab', { name: /overview/i }).click();
    await page.screenshot({ path: `${DOCS_IMAGES}/entry-detail-overview-tab.png`, fullPage: false });
  });

  test('entry-detail-a2a-skills', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto(`/catalog/${a2aEntryId}`);
    await page.waitForLoadState('networkidle');
    // Scroll to capabilities section
    await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
    await page.screenshot({ path: `${DOCS_IMAGES}/entry-detail-a2a-skills.png`, fullPage: false });
  });

  test('entry-detail-mcp', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto(`/catalog/${mcpEntryId}`);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/entry-detail-mcp.png`, fullPage: false });
  });

  test('entry-detail-raw-json', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto(`/catalog/${a2aEntryId}`);
    await page.waitForLoadState('networkidle');
    // Click the Raw Card tab to reveal syntax-highlighted JSON
    await page.getByRole('tab', { name: /raw card/i }).click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/entry-detail-raw-json.png`, fullPage: false });
  });

  test('entry-detail-raw-card-tab', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto(`/catalog/${a2aEntryId}`);
    await page.waitForLoadState('networkidle');
    // Click the Raw Card tab to reveal syntax-highlighted JSON, Copy, and Download buttons
    await page.getByRole('tab', { name: /raw card/i }).click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/entry-detail-raw-card-tab.png`, fullPage: false });
  });

  // ───────── Register Agent screenshots ─────────

  test('register-menu', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.waitForLoadState('networkidle');
    // Show the Register Agent button — just scroll to top to keep it in frame
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({ path: `${DOCS_IMAGES}/register-menu.png`, fullPage: false });
  });

  test('register-import-url-empty', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    // Open dialog
    await page.getByRole('button', { name: 'Register Agent' }).click();
    // Switch to Import from URL tab
    await page.getByRole('tab', { name: /import from url/i }).click();
    await page.screenshot({ path: `${DOCS_IMAGES}/register-import-url-empty.png`, fullPage: false });
  });

  test('register-import-url-filled', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.getByRole('button', { name: 'Register Agent' }).click();
    await page.getByRole('tab', { name: /import from url/i }).click();
    // Fill in a public URL
    await page.getByPlaceholder(/https:\/\/example\.com/i).fill(DEMO_IMPORT_URL);
    await page.screenshot({ path: `${DOCS_IMAGES}/register-import-url-filled.png`, fullPage: false });
  });

  test('register-import-url-error-private', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.getByRole('button', { name: 'Register Agent' }).click();
    await page.getByRole('tab', { name: /import from url/i }).click();
    // Use a private/loopback URL — SSRF guard should reject it
    await page.getByPlaceholder(/https:\/\/example\.com/i).fill('http://127.0.0.1:9999/agent.json');
    await page.getByRole('button', { name: /fetch.*import/i }).click();
    // Wait for error to appear
    await page.waitForSelector('.text-destructive', { timeout: ASYNC_OP_TIMEOUT_MS });
    await page.screenshot({ path: `${DOCS_IMAGES}/register-import-url-error-private.png`, fullPage: false });
  });

  test('register-import-url-success', async ({ page }) => {
    // This screenshot shows the catalog after a successful import.
    // We'll register via the API (to keep it fast & reliable), then screenshot the dashboard.
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.waitForLoadState('networkidle');
    // The dashboard is already showing the seeded entries — use that as the "success" view
    await page.screenshot({ path: `${DOCS_IMAGES}/register-import-url-success.png`, fullPage: false });
  });

  test('register-paste-json-empty', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.getByRole('button', { name: 'Register Agent' }).click();
    // Paste JSON is the default tab
    await page.screenshot({ path: `${DOCS_IMAGES}/register-paste-json-empty.png`, fullPage: false });
  });

  test('register-paste-json-validation', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.getByRole('button', { name: 'Register Agent' }).click();
    // Paste invalid JSON to trigger syntax error, then submit
    const schemaInvalidCard = JSON.stringify({ foo: 'bar' }, null, 2);
    await page.locator('textarea').fill(schemaInvalidCard);
    await page.getByRole('button', { name: 'Validate' }).click();
    // Wait for validation result
    await page.waitForSelector('.text-destructive, .border-destructive', { timeout: ASYNC_OP_TIMEOUT_MS });
    await page.screenshot({ path: `${DOCS_IMAGES}/register-paste-json-validation.png`, fullPage: false });
  });

  test('register-paste-json-preview', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.getByRole('button', { name: 'Register Agent' }).click();
    // Paste a valid A2A card
    await page.locator('textarea').fill(A2A_CARD);
    await page.getByRole('button', { name: 'Validate' }).click();
    // Wait for preview step
    await page.waitForSelector('button:has-text("Register Agent"):not([aria-label])', { timeout: ASYNC_OP_TIMEOUT_MS });
    await page.screenshot({ path: `${DOCS_IMAGES}/register-paste-json-preview.png`, fullPage: false });
  });

  test('register-success-toast', async ({ page }) => {
    // After a successful register the dialog closes and the catalog refreshes.
    // We use the populated dashboard as the success state.
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/register-success-toast.png`, fullPage: false });
  });

  // ───────── Settings screenshots ─────────

  test('settings-general', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/settings-general.png`, fullPage: true });
  });

  test('settings-account', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto('/settings?tab=account');
    await page.waitForLoadState('networkidle');
    // Click the My Account tab
    await page.getByRole('tab', { name: /my account/i }).click();
    await page.screenshot({ path: `${DOCS_IMAGES}/settings-account.png`, fullPage: true });
  });

  test('settings-users-list', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await page.getByRole('tab', { name: /users/i }).click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/settings-users-list.png`, fullPage: true });
  });

  test('settings-users-create', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto('/settings');
    await page.getByRole('tab', { name: /users/i }).click();
    await page.waitForLoadState('networkidle');
    // Open the Add user dialog
    await page.getByRole('button', { name: /add user/i }).click();
    await page.waitForSelector('[role="dialog"]');
    await page.screenshot({ path: `${DOCS_IMAGES}/settings-users-create.png`, fullPage: false });
  });

  test('settings-users-locked', async ({ page, request }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    // Lock the demo_viewer user by setting is_active=false via API
    if (viewerUserId) {
      await request.put(`${BASE}/api/v1/users/${viewerUserId}`, {
        headers: authHeader(token),
        data: { is_active: false },
      });
    }
    await loginViaUI(page);
    await page.goto('/settings');
    await page.getByRole('tab', { name: /users/i }).click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/settings-users-locked.png`, fullPage: true });
    // Unlock again for cleanliness
    if (viewerUserId) {
      await request.put(`${BASE}/api/v1/users/${viewerUserId}`, {
        headers: authHeader(token),
        data: { is_active: true },
      }).catch(ignoreCleanupError('unlock viewer user'));
    }
  });

  test('settings-roles-list', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto('/settings');
    await page.getByRole('tab', { name: /roles/i }).click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/settings-roles-list.png`, fullPage: true });
  });

  test('settings-roles-edit', async ({ page, request }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });

    // Create a custom role for this screenshot (system roles are not editable)
    const rolesRes = await request.post(`${BASE}/api/v1/roles`, {
      headers: authHeader(token),
      data: {
        name: 'Demo Operator',
        description: 'Operator role for documentation screenshot',
        permissions: ['catalog:read', 'catalog:write'],
      },
    });
    const customRole = await rolesRes.json();

    await loginViaUI(page);
    await page.goto('/settings');
    await page.getByRole('tab', { name: /roles/i }).click();
    await page.waitForLoadState('networkidle');
    // Click edit on the custom role
    const editButtons = page.getByTitle('Edit');
    const count = await editButtons.count();
    if (count > 0) {
      await editButtons.last().click();
      await page.waitForSelector('[role="dialog"]');
    }
    await page.screenshot({ path: `${DOCS_IMAGES}/settings-roles-edit.png`, fullPage: false });

    // Clean up custom role
    await request.delete(`${BASE}/api/v1/roles/${customRole.id}`, {
      headers: authHeader(token),
    }).catch(ignoreCleanupError('delete custom role'));
  });

  // ───────── Health lifecycle screenshots ─────────

  test('health-status-badges', async ({ page }) => {
    // Shows the catalog list with the new lifecycle status badges visible.
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.waitForLoadState('networkidle');
    // Wait for at least one StatusBadge to render — Badge renders as a div with cursor-default
    await page.locator('.cursor-default').first().waitFor({ timeout: ASYNC_OP_TIMEOUT_MS });
    await page.screenshot({ path: `${DOCS_IMAGES}/health-status-badges.png`, fullPage: false });
  });

  test('health-filter-multi-select', async ({ page }) => {
    // Status is now shown per-row in the catalog table — show the catalog list
    // with both A2A and MCP entries visible to illustrate the status column.
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/health-filter-multi-select.png`, fullPage: false });
  });

  test('health-filter-active-selected', async ({ page }) => {
    // Show catalog filtered to the A2A protocol to illustrate narrowing entries
    // (status filtering from the list is now done via the Status column in the table).
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.waitForLoadState('networkidle');
    // Use the protocol toggle to narrow to A2A entries
    await page.getByRole('radio', { name: 'A2A' }).click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/health-filter-active-selected.png`, fullPage: false });
  });

  test('health-detail-section', async ({ page }) => {
    // Shows the Health section in an entry detail view.
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto(`/catalog/${a2aEntryId}`);
    await page.waitForLoadState('networkidle');
    // Scroll to the Health section
    await page.evaluate(() => {
      const headings = Array.from(document.querySelectorAll('h3'));
      const healthHeading = headings.find(h => /health/i.test(h.textContent ?? ''));
      healthHeading?.scrollIntoView({ behavior: 'instant', block: 'center' });
    });
    await page.screenshot({ path: `${DOCS_IMAGES}/health-detail-section.png`, fullPage: false });
  });

  test('health-probe-now', async ({ page }) => {
    // Shows the Probe now button in the health section (before clicking).
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto(`/catalog/${a2aEntryId}`);
    await page.waitForLoadState('networkidle');
    await page.evaluate(() => {
      const headings = Array.from(document.querySelectorAll('h3'));
      const healthHeading = headings.find(h => /health/i.test(h.textContent ?? ''));
      healthHeading?.scrollIntoView({ behavior: 'instant', block: 'center' });
    });
    // Highlight the Probe now button by hovering
    await page.getByRole('button', { name: /probe now/i }).hover();
    await page.screenshot({ path: `${DOCS_IMAGES}/health-probe-now.png`, fullPage: false });
  });

  test('health-deprecate-dialog', async ({ page }) => {
    // Shows the Deprecate button in the detail page header (new UI — no confirmation
    // dialog; clicking directly applies the lifecycle change).
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto(`/catalog/${a2aEntryId}`);
    await page.waitForLoadState('networkidle');
    // Hover the Deprecate button to show the focused/hover state for documentation.
    await page.getByRole('button', { name: /deprecate/i }).hover();
    await page.screenshot({ path: `${DOCS_IMAGES}/health-deprecate-dialog.png`, fullPage: false });
  });

  test('health-deprecated-badge', async ({ page, request }) => {
    // Shows an entry with the Deprecated badge.
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    // Get a fresh token for the API calls in this test — avoids stale-token issues on retry.
    const freshToken = await loginViaAPI(request);
    // Deprecate the A2A entry via API for this screenshot (MCP entry may be unavailable on retry).
    await request.patch(`${BASE}/api/v1/catalog/${a2aEntryId}/lifecycle`, {
      headers: authHeader(freshToken),
      data: { state: 'deprecated' },
    });
    await loginViaUI(page);
    await page.goto(`/catalog/${a2aEntryId}`);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/health-deprecated-badge.png`, fullPage: false });
    // Restore to registered state so subsequent tests are not affected.
    await request.patch(`${BASE}/api/v1/catalog/${a2aEntryId}/lifecycle`, {
      headers: authHeader(freshToken),
      data: { state: 'registered' },
    }).catch(ignoreCleanupError('un-deprecate a2a entry'));
  });

  // ───────── User dropdown ─────────

  test('user-dropdown', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    // Open the user dropdown — target the trigger button by its initials div child
    await page.locator('header button').filter({ has: page.locator('.rounded-full') }).click();
    await page.waitForSelector('[role="menu"]');
    await page.screenshot({ path: `${DOCS_IMAGES}/user-dropdown.png`, fullPage: false });
  });

  // ───────── Capabilities screenshots ─────────

  test('capabilities-list', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto('/catalog/capabilities');
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/capabilities-list.png`, fullPage: false });
  });

  test('capabilities-list-expanded', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto('/catalog/capabilities');
    await page.waitForLoadState('networkidle');
    // Each capability group is a div.border.rounded-lg; its first child is the toggle button.
    // Click the first group's header button to expand it.
    const groupButton = page.locator('div.border.rounded-lg > button').first();
    await groupButton.click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/capabilities-list-expanded.png`, fullPage: false });
  });

  test('capabilities-kind-filter', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto('/catalog/capabilities');
    await page.waitForLoadState('networkidle');
    // Click "A2A Skill" in the KindFilter (ToggleGroup renders items as role="radio")
    await page.getByRole('radio', { name: 'A2A Skill' }).click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/capabilities-kind-filter.png`, fullPage: false });
  });

  test('capabilities-search', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto('/catalog/capabilities');
    await page.waitForLoadState('networkidle');
    // Type a search query
    await page.getByPlaceholder(/search/i).first().fill('Translate');
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/capabilities-search.png`, fullPage: false });
  });

  test('capabilities-detail', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    // Navigate directly to the Translate capability detail page
    const key = encodeURIComponent('a2a.skill::Translate');
    await page.goto(`/catalog/capabilities/${key}`);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/capabilities-detail.png`, fullPage: false });
  });

  test('capabilities-crosslink', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    // Navigate to the A2A entry that was registered with capabilities
    await page.goto(`/catalog/${a2aCapEntryId}`);
    await page.waitForLoadState('networkidle');
    // Scroll to the capabilities / skills section to show the clickable links
    await page.evaluate(() => {
      const headings = Array.from(document.querySelectorAll('h2, h3'));
      const capHeading = headings.find(h =>
        /capabilities|skills/i.test(h.textContent ?? '')
      );
      capHeading?.scrollIntoView({ behavior: 'instant', block: 'center' });
    });
    await page.screenshot({ path: `${DOCS_IMAGES}/capabilities-crosslink.png`, fullPage: false });
  });

  // ───────── Project filter screenshots ─────────

  test('catalog-project-filter', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    // Navigate to the catalog filtered to the seeded project.
    // Shows only the entries assigned to that project.
    await page.goto(`/catalog?project=${projectId}`);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/catalog-project-filter.png`, fullPage: false });
  });

  test('catalog-entry-projects', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    // Navigate to the A2A entry detail page — shows the entry's project memberships.
    await page.goto(`/catalog/${a2aEntryId}`);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/catalog-entry-projects.png`, fullPage: false });
  });

  // ───────── Groups screenshots ─────────

  test('groups-tab', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto('/settings');
    await page.getByRole('tab', { name: /groups/i }).click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/groups-tab.png`, fullPage: false });
  });

  test('group-detail', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto(`/settings/groups/${demoGroupId}`);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/group-detail.png`, fullPage: false });
  });

  // ───────── Projects screenshots ─────────

  test('projects-tab', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto('/settings');
    await page.getByRole('tab', { name: /^projects$/i }).click();
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/projects-tab.png`, fullPage: false });
  });

  test('project-detail', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await loginViaUI(page);
    await page.goto(`/settings/projects/${projectId}`);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: `${DOCS_IMAGES}/project-detail.png`, fullPage: false });
  });
});

// Export nothing — this file is a generator, not a module with exports.
export {};

/**
 * E2E tests for A2A security scheme display.
 *
 * Covers:
 * - Authentication section on agent detail (schemes, requirements banner, connection recipe)
 * - "No auth" state for open agents
 * - MCP auth message for MCP servers
 * - Auth badge in catalog list
 * - Documentation screenshots (saved to docs/images/)
 */
import { test, expect } from '@playwright/test';
import path from 'path';
import { fileURLToPath } from 'url';
import {
  loginViaAPI,
  loginViaUI,
  deleteCatalogEntry,
  authHeader,
  BASE,
} from './helpers';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const DOCS_IMAGES = path.resolve(__dirname, '../../docs/images');

// ──────────────────────────────────────────────────────────────
// Helpers to seed agents via /register (parses capabilities)
// ──────────────────────────────────────────────────────────────

async function registerCard(
  request: Parameters<typeof deleteCatalogEntry>[0],
  token: string,
  card: object,
): Promise<string> {
  const res = await request.post(`${BASE}/api/v1/catalog/register`, {
    headers: { ...authHeader(token), 'Content-Type': 'application/json' },
    data: JSON.stringify(card),
  });
  if (res.ok()) {
    const body = await res.json();
    return body.id as string;
  }
  if (res.status() === 409) {
    // Already registered — look up by endpoint (deterministic) or display_name
    const allRes = await request.get(`${BASE}/api/v1/catalog`, {
      headers: authHeader(token),
    });
    const all: Array<{ id: string; display_name: string; endpoint: string }> = await allRes.json();
    const name = (card as { name: string }).name;
    const url = (card as { url: string }).url;
    const existing =
      all.find((e) => e.endpoint === url) ?? all.find((e) => e.display_name === name);
    if (existing) return existing.id;
    throw new Error(`registerCard 409 but entry not found by endpoint=${url} or name=${name}`);
  }
  const body = await res.text();
  throw new Error(`registerCard failed: ${res.status()} ${body}`);
}

// ──────────────────────────────────────────────────────────────
// Test cards
// ──────────────────────────────────────────────────────────────

const BEARER_APIKEY_CARD = {
  version: '1.0',
  name: 'E2E Bearer+ApiKey Agent',
  description: 'Agent with bearer and API key auth schemes for E2E tests',
  url: 'https://e2e-bearer-apikey.example.com',
  securitySchemes: {
    httpAuth: {
      type: 'http',
      scheme: 'Bearer',
      bearerFormat: 'JWT',
      description: 'JWT Bearer token',
    },
    apiKeyAuth: {
      type: 'apiKey',
      in: 'header',
      name: 'X-API-Key',
      description: 'API Key authentication',
    },
  },
  securityRequirements: [{ httpAuth: [] }, { apiKeyAuth: [] }],
  skills: [],
};

const OPEN_CARD = {
  version: '1.0',
  name: 'E2E Open Agent',
  description: 'Agent with no security for E2E tests',
  url: 'https://e2e-open-agent.example.com',
  skills: [],
};

const OAUTH2_CARD = {
  version: '1.0',
  name: 'E2E OAuth2 Agent',
  description: 'Agent with OAuth2 authorization code flow for E2E tests',
  url: 'https://e2e-oauth2.example.com',
  securitySchemes: {
    oauth2Auth: {
      type: 'oauth2',
      flows: {
        authorizationCode: {
          authorizationUrl: 'https://auth.example.com/authorize',
          tokenUrl: 'https://auth.example.com/token',
          scopes: {
            'read:agents': 'Read agent catalog',
            'write:agents': 'Register agents',
          },
        },
      },
    },
  },
  securityRequirements: [{ oauth2Auth: ['read:agents'] }],
  skills: [],
};

// ──────────────────────────────────────────────────────────────
// Suite
// ──────────────────────────────────────────────────────────────

test.describe('A2A Security Schemes Display', () => {
  let token: string;
  let bearerApiKeyId: string;
  let openAgentId: string;
  let mcpAgentId: string;

  test.beforeAll(async ({ request }) => {
    token = await loginViaAPI(request);

    // Seed bearer+apiKey A2A agent
    bearerApiKeyId = await registerCard(request, token, BEARER_APIKEY_CARD);

    // Seed open A2A agent
    openAgentId = await registerCard(request, token, OPEN_CARD);

    // Seed MCP agent via /catalog (no card parsing needed)
    const mcpEndpoint = 'https://e2e-mcp-server.example.com';
    const mcpRes = await request.post(`${BASE}/api/v1/catalog`, {
      headers: authHeader(token),
      data: {
        display_name: 'E2E MCP Server',
        description: 'MCP server for E2E auth display test',
        protocol: 'mcp',
        endpoint: mcpEndpoint,
      },
    });
    if (mcpRes.ok()) {
      const mcp = await mcpRes.json();
      mcpAgentId = mcp.id as string;
    } else if (mcpRes.status() === 409) {
      // Already exists — find by endpoint
      const allRes = await request.get(`${BASE}/api/v1/catalog`, { headers: authHeader(token) });
      const all: Array<{ id: string; endpoint: string }> = await allRes.json();
      const existing = all.find((e) => e.endpoint === mcpEndpoint);
      if (existing) mcpAgentId = existing.id;
      else throw new Error(`MCP seed 409 but not found by endpoint`);
    } else {
      throw new Error(`MCP seed failed: ${mcpRes.status()} ${await mcpRes.text()}`);
    }
  });

  test.afterAll(async ({ request }) => {
    for (const id of [bearerApiKeyId, openAgentId, mcpAgentId]) {
      if (id) await deleteCatalogEntry(request, token, id);
    }
  });

  // ── Test 1: bearer + apiKey agent shows full auth section ──

  test('displays security schemes on agent detail', async ({ page }) => {
    await loginViaUI(page);
    await page.goto(`/catalog/${bearerApiKeyId}`);
    await page.waitForLoadState('networkidle');

    // Authentication section is present
    const authSection = page.getByTestId('authentication-section');
    await expect(authSection).toBeVisible({ timeout: 10_000 });

    // Section heading
    await expect(authSection.getByRole('heading', { name: 'Authentication' })).toBeVisible();

    // Security requirements banner is rendered (multiple top-level requirements)
    const banner = page.getByTestId('security-requirements-banner');
    await expect(banner).toBeVisible();
    await expect(banner.getByText('Required to connect')).toBeVisible();

    // Scheme names appear somewhere in the auth section (may appear in banner + scheme cards)
    await expect(authSection.getByText('httpAuth').first()).toBeVisible();
    await expect(authSection.getByText('apiKeyAuth').first()).toBeVisible();

    // Connection recipe is shown (endpoint + requirements present)
    const recipe = page.getByTestId('connection-recipe');
    await expect(recipe).toBeVisible();
    await expect(recipe.getByText('Connection Example')).toBeVisible();
    // curl snippet should contain the endpoint
    await expect(recipe.locator('code')).toContainText('curl');
  });

  // ── Test 2: open agent shows "no auth" message ──

  test('displays "no auth" state for open agents', async ({ page }) => {
    await loginViaUI(page);
    await page.goto(`/catalog/${openAgentId}`);
    await page.waitForLoadState('networkidle');

    const authSection = page.getByTestId('authentication-section');
    await expect(authSection).toBeVisible({ timeout: 10_000 });

    // Should say no authentication requirements
    await expect(
      authSection.getByText('This agent does not declare any authentication requirements.'),
    ).toBeVisible();

    // No scheme cards, no banner, no recipe
    await expect(page.getByTestId('security-requirements-banner')).not.toBeVisible();
    await expect(page.getByTestId('connection-recipe')).not.toBeVisible();
  });

  // ── Test 3: MCP agent shows transport-level message ──

  test('displays MCP auth message for MCP servers', async ({ page }) => {
    await loginViaUI(page);
    await page.goto(`/catalog/${mcpAgentId}`);
    await page.waitForLoadState('networkidle');

    const authSection = page.getByTestId('authentication-section');
    await expect(authSection).toBeVisible({ timeout: 10_000 });

    await expect(
      authSection.getByText(
        'MCP servers declare authentication at the transport level, not in the server card.',
      ),
    ).toBeVisible();
  });

  // ── Test 4: auth badge in catalog list ──

  test('displays auth badge in catalog list', async ({ page }) => {
    await loginViaUI(page);
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Ensure the Auth column header is present
    await expect(page.getByRole('columnheader', { name: 'Auth' })).toBeVisible({ timeout: 10_000 });

    // The bearer+apiKey agent row should show an auth badge with scheme info
    const searchInput = page.getByRole('textbox', { name: 'Search catalog' });
    await searchInput.fill('E2E Bearer');
    await page.waitForTimeout(400);

    const row = page.locator('tr', { hasText: 'E2E Bearer+ApiKey Agent' });
    await expect(row).toBeVisible({ timeout: 5_000 });
    // The auth badge text should NOT be the "no auth" placeholder
    await expect(row).not.toContainText('Open (no auth)');
    // The auth cell should contain some auth scheme text (API Key, Bearer JWT, etc.)
    // The exact label depends on computed auth_summary; just verify it's non-empty and not "no auth"
    const authCell = row.locator('td').nth(6); // Auth is the 7th column (0-indexed: 6)
    await expect(authCell).not.toBeEmpty();

    // Open agent has no security schemes so auth_summary is null — Auth cell is empty
    await searchInput.fill('E2E Open Agent');
    await page.waitForTimeout(400);
    const openRow = page.locator('tr', { hasText: 'E2E Open Agent' });
    await expect(openRow).toBeVisible({ timeout: 5_000 });
    // Auth cell is empty — no badge rendered for open agents
    const openAuthCell = openRow.locator('td').nth(6);
    await expect(openAuthCell).toBeEmpty();
  });
})

// ──────────────────────────────────────────────────────────────
// Documentation Screenshots
// Saves to docs/images/ for use in end-user-guide.md
// ──────────────────────────────────────────────────────────────

const BEARER_APIKEY_CARD_DOCS = {
  ...BEARER_APIKEY_CARD,
  name: 'E2E Bearer+ApiKey Agent (docs)',
  url: 'https://e2e-bearer-apikey-docs.example.com',
};

const OPEN_CARD_DOCS = {
  ...OPEN_CARD,
  name: 'E2E Open Agent (docs)',
  url: 'https://e2e-open-agent-docs.example.com',
};

const OAUTH2_CARD_DOCS = {
  ...OAUTH2_CARD,
  name: 'E2E OAuth2 Agent (docs)',
  url: 'https://e2e-oauth2-docs.example.com',
};

test.describe('A2A Security — Documentation Screenshots', () => {
  const VIEWPORT = { width: 1680, height: 1050 };

  let token: string;
  let bearerApiKeyId: string;
  let openAgentId: string;
  let oauth2AgentId: string;

  test.beforeAll(async ({ request }) => {
    token = await loginViaAPI(request);
    bearerApiKeyId = await registerCard(request, token, BEARER_APIKEY_CARD_DOCS);
    openAgentId = await registerCard(request, token, OPEN_CARD_DOCS);
    oauth2AgentId = await registerCard(request, token, OAUTH2_CARD_DOCS);
  });

  test.afterAll(async ({ request }) => {
    for (const id of [bearerApiKeyId, openAgentId, oauth2AgentId]) {
      if (id) await deleteCatalogEntry(request, token, id);
    }
  });

  test('screenshot: detail page — bearer auth', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await loginViaUI(page);
    await page.goto(`/catalog/${bearerApiKeyId}`);
    await page.waitForLoadState('networkidle');
    await expect(page.getByTestId('authentication-section')).toBeVisible({ timeout: 10_000 });
    await page.getByTestId('authentication-section').scrollIntoViewIfNeeded();
    await page.screenshot({
      path: `${DOCS_IMAGES}/security-detail-bearer.png`,
      fullPage: false,
    });
  });

  test('screenshot: detail page — oauth2', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await loginViaUI(page);
    await page.goto(`/catalog/${oauth2AgentId}`);
    await page.waitForLoadState('networkidle');
    await expect(page.getByTestId('authentication-section')).toBeVisible({ timeout: 10_000 });
    await page.getByTestId('authentication-section').scrollIntoViewIfNeeded();
    await page.screenshot({
      path: `${DOCS_IMAGES}/security-detail-oauth2.png`,
      fullPage: false,
    });
  });

  test('screenshot: detail page — no auth', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await loginViaUI(page);
    await page.goto(`/catalog/${openAgentId}`);
    await page.waitForLoadState('networkidle');
    await expect(page.getByTestId('authentication-section')).toBeVisible({ timeout: 10_000 });
    await page.getByTestId('authentication-section').scrollIntoViewIfNeeded();
    await page.screenshot({
      path: `${DOCS_IMAGES}/security-detail-no-auth.png`,
      fullPage: false,
    });
  });

  test('screenshot: catalog list — auth badges', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await loginViaUI(page);
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('columnheader', { name: 'Auth' })).toBeVisible({ timeout: 10_000 });
    // Search for E2E docs agents so they're visible together
    const searchInput = page.getByRole('textbox', { name: 'Search catalog' });
    await searchInput.fill('E2E Bearer+ApiKey Agent (docs)');
    await page.waitForTimeout(500);
    await page.screenshot({
      path: `${DOCS_IMAGES}/catalog-list-auth-badges.png`,
      fullPage: false,
    });
  });

  test('screenshot: security requirements banner', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await loginViaUI(page);
    await page.goto(`/catalog/${bearerApiKeyId}`);
    await page.waitForLoadState('networkidle');
    const banner = page.getByTestId('security-requirements-banner');
    await expect(banner).toBeVisible({ timeout: 10_000 });
    await banner.scrollIntoViewIfNeeded();
    await banner.screenshot({
      path: `${DOCS_IMAGES}/security-requirements-banner.png`,
    });
  });

  test('screenshot: connection recipe', async ({ page }) => {
    await page.setViewportSize(VIEWPORT);
    await loginViaUI(page);
    await page.goto(`/catalog/${bearerApiKeyId}`);
    await page.waitForLoadState('networkidle');
    const recipe = page.getByTestId('connection-recipe');
    await expect(recipe).toBeVisible({ timeout: 10_000 });
    await recipe.scrollIntoViewIfNeeded();
    await recipe.screenshot({
      path: `${DOCS_IMAGES}/connection-recipe.png`,
    });
  });
});

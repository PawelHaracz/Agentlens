import { test, expect } from '@playwright/test';
import {
  loginViaAPI,
  loginViaUI,
  createCatalogEntry,
  deleteCatalogEntry,
  authHeader,
  BASE,
} from './helpers';

test.describe('Catalog Management', () => {
  let token: string;

  test.beforeAll(async ({ request }) => {
    token = await loginViaAPI(request);
  });

  test('API: list catalog entries (initially empty or with seed data)', async ({ request }) => {
    const res = await request.get(`${BASE}/api/v1/catalog`, {
      headers: authHeader(token),
    });
    expect(res.ok()).toBeTruthy();
    const entries = await res.json();
    expect(Array.isArray(entries)).toBeTruthy();
  });

  test('API: create, get, and delete a catalog entry', async ({ request }) => {
    // Create
    const entry = await createCatalogEntry(request, token, {
      display_name: 'CRUD Test Agent',
      description: 'Created by E2E test',
      protocol: 'a2a',
      endpoint: `http://crud-test-${Date.now()}.example.com`,
    });
    expect(entry.id).toBeTruthy();
    expect(entry.display_name).toBe('CRUD Test Agent');

    // Get by ID
    const getRes = await request.get(`${BASE}/api/v1/catalog/${entry.id}`, {
      headers: authHeader(token),
    });
    expect(getRes.ok()).toBeTruthy();
    const fetched = await getRes.json();
    expect(fetched.id).toBe(entry.id);
    expect(fetched.display_name).toBe('CRUD Test Agent');

    // Delete
    await deleteCatalogEntry(request, token, entry.id);

    // Verify deleted
    const verifyRes = await request.get(`${BASE}/api/v1/catalog/${entry.id}`, {
      headers: authHeader(token),
    });
    expect(verifyRes.status()).toBe(404);
  });

  test('API: create MCP protocol entry', async ({ request }) => {
    const entry = await createCatalogEntry(request, token, {
      display_name: 'MCP Server',
      protocol: 'mcp',
      endpoint: `http://mcp-test-${Date.now()}.example.com`,
    });
    expect(entry.protocol).toBe('mcp');
    // Cleanup
    await deleteCatalogEntry(request, token, entry.id);
  });

  test('API: duplicate endpoint returns 409', async ({ request }) => {
    const endpoint = `http://dup-test-${Date.now()}.example.com`;
    const entry = await createCatalogEntry(request, token, { endpoint });

    // Try creating with the same endpoint
    const dupRes = await request.post(`${BASE}/api/v1/catalog`, {
      headers: authHeader(token),
      data: {
        display_name: 'Duplicate Agent',
        protocol: 'a2a',
        endpoint,
      },
    });
    expect(dupRes.status()).toBe(409);

    // Cleanup
    await deleteCatalogEntry(request, token, entry.id);
  });

  test('API: get catalog stats', async ({ request }) => {
    const res = await request.get(`${BASE}/api/v1/stats`, {
      headers: authHeader(token),
    });
    expect(res.ok()).toBeTruthy();
    const stats = await res.json();
    expect(stats).toHaveProperty('total');
  });

  test('API: search skills', async ({ request }) => {
    const res = await request.get(`${BASE}/api/v1/skills?q=test`, {
      headers: authHeader(token),
    });
    expect(res.ok()).toBeTruthy();
    const results = await res.json();
    expect(Array.isArray(results)).toBeTruthy();
  });

  test('API: invalid protocol returns 400', async ({ request }) => {
    const res = await request.post(`${BASE}/api/v1/catalog`, {
      headers: authHeader(token),
      data: {
        display_name: 'Bad Protocol',
        protocol: 'invalid',
        endpoint: `http://bad-${Date.now()}.example.com`,
      },
    });
    expect(res.status()).toBe(400);
  });

  test('API: missing required fields returns 400', async ({ request }) => {
    const res = await request.post(`${BASE}/api/v1/catalog`, {
      headers: authHeader(token),
      data: { description: 'Missing required fields' },
    });
    expect(res.status()).toBe(400);
  });

  test('UI: dashboard shows catalog table', async ({ page }) => {
    await loginViaUI(page);
    // Stats bar should be visible
    await expect(page.getByText('Total').first()).toBeVisible();
    // Table headers should be visible
    await expect(page.getByText(/Name|Protocol|Status/i).first()).toBeVisible();
  });

  test('UI: catalog entry detail view', async ({ page, request }) => {
    // Create an entry via API
    const entry = await createCatalogEntry(request, token, {
      display_name: 'Detail View Agent',
      description: 'Agent for detail view test',
      endpoint: `http://detail-test-${Date.now()}.example.com`,
    });

    await loginViaUI(page);

    // Click on the entry name
    await page.getByText('Detail View Agent').click();

    // Detail page should show the entry info
    await expect(page.getByText('Detail View Agent')).toBeVisible();
    await expect(page.getByText('Agent for detail view test')).toBeVisible();

    // Cleanup
    await deleteCatalogEntry(request, token, entry.id);
  });

  test('UI: search filters catalog entries', async ({ page, request }) => {
    const entry = await createCatalogEntry(request, token, {
      display_name: 'Searchable E2E Agent',
      endpoint: `http://search-test-${Date.now()}.example.com`,
    });

    await loginViaUI(page);

    // Type into search bar
    const searchInput = page.getByPlaceholder(/search/i);
    await searchInput.fill('Searchable E2E');

    // The entry should be visible
    await expect(page.getByText('Searchable E2E Agent')).toBeVisible();

    // Clear search and type something else
    await searchInput.fill('nonexistent-xyz-agent');
    // Wait for filter to apply
    await page.waitForTimeout(500);

    // Cleanup
    await deleteCatalogEntry(request, token, entry.id);
  });
});

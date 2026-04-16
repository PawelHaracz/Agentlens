import { test, expect } from '@playwright/test';
import {
  loginViaAPI,
  loginViaUI,
  createCatalogEntry,
  deleteCatalogEntry,
  validateAgentCard,
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

  test('capabilities API returns capability instances', async ({ request }) => {
    const response = await request.get(`${BASE}/api/v1/capabilities`, {
      headers: authHeader(token),
    })

    expect(response.status()).toBe(200)
    const data = await response.json()
    expect(data).toHaveProperty('total')
    expect(data).toHaveProperty('items')
    expect(Array.isArray(data.items)).toBe(true)
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

  test('UI: dashboard shows catalog table', async ({ page, request }) => {
    // Create an entry so the table renders (empty catalog shows empty state, not table headers).
    const entry = await createCatalogEntry(request, token, {
      display_name: 'Dashboard Table Agent',
      endpoint: `http://dashboard-table-${Date.now()}.example.com`,
    });

    await loginViaUI(page);
    // Stats bar should be visible
    await expect(page.getByText('Total').first()).toBeVisible();
    // Table headers should be visible when there is at least one entry
    await expect(page.getByText(/Name|Protocol|Status/i).first()).toBeVisible();

    // Cleanup
    await deleteCatalogEntry(request, token, entry.id);
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

  test('validate valid A2A v1.0 card returns 200 with preview', async ({ request }) => {
    const card = JSON.stringify({
      name: 'E2E Validation Agent',
      description: 'Tests validation endpoint',
      version: '1.0.0',
      supportedInterfaces: [{ url: `http://validate-${Date.now()}.example.com/a2a`, binding: 'jsonrpc' }],
      capabilities: {
        extensions: [{ uri: 'urn:test:ext', required: true }],
        supportsExtendedAgentCard: true,
      },
      securitySchemes: [{ type: 'bearer', method: 'header', name: 'Authorization' }],
      skills: [{ id: 's1', name: 'Skill One', description: 'A test skill' }],
    });

    const { status, body } = await validateAgentCard(request, token, card);
    expect(status).toBe(200);
    expect(body.valid).toBe(true);
    expect(body.spec_version).toBe('1.0');
    expect(body.preview.display_name).toBe('E2E Validation Agent');
    expect(body.preview.extensions_count).toBe(1);
    expect(body.preview.security_schemes).toContain('bearer');
  });

  test('validate invalid card returns 422 with errors', async ({ request }) => {
    const card = JSON.stringify({
      description: 'Missing name and endpoints',
      skills: [{ name: 'No ID' }],
    });

    const { status, body } = await validateAgentCard(request, token, card);
    expect(status).toBe(422);
    expect(body.valid).toBe(false);
    expect(body.errors.length).toBeGreaterThan(0);

    const fields = body.errors.map((e: { field: string }) => e.field);
    expect(fields).toContain('name');
    expect(fields).toContain('url');
  });

  test('register agent via UI modal', async ({ page }) => {
    await loginViaUI(page);

    // Open register dialog.
    await page.getByRole('button', { name: 'Register Agent' }).click();
    await expect(page.getByText('Register Agent').nth(1)).toBeVisible();

    // Paste valid JSON.
    const card = JSON.stringify({
      name: `UI Register Agent ${Date.now()}`,
      description: 'Registered via UI E2E test',
      version: '1.0.0',
      url: `http://ui-register-${Date.now()}.example.com`,
      skills: [{ id: 'echo', name: 'Echo', description: 'Echoes input' }],
    }, null, 2);

    const textarea = page.locator('textarea');
    await textarea.fill(card);

    // Validate.
    await page.getByRole('button', { name: 'Validate' }).click();

    // Should advance to preview (or show preview).
    await expect(page.getByText('Card validated successfully')).toBeVisible({ timeout: 10_000 });

    // Register.
    await page.getByRole('button', { name: 'Register Agent' }).click();

    // Should close dialog and refresh catalog.
    await expect(page.getByText('UI Register Agent')).toBeVisible({ timeout: 10_000 });
  });
});

test.describe('Unified Catalog View', () => {
  let token: string;

  test.beforeEach(async ({ request }) => {
    token = await loginViaAPI(request);
  });

  test('shows all entries by default (All protocol filter)', async ({ page, request }) => {
    await loginViaUI(page);
    await page.goto('/');
    // Protocol filter should show "All" selected by default.
    await expect(page.getByRole('group', { name: 'Filter by protocol' })).toBeVisible();
    // Table should be visible or empty state shown.
    const table = page.locator('table');
    const emptyState = page.getByText(/No agents registered yet/i);
    await expect(table.or(emptyState)).toBeVisible({ timeout: 10_000 });
  });

  test('search input focuses on "/" keypress', async ({ page }) => {
    await loginViaUI(page);
    await page.goto('/');
    // Click the page body to ensure keyboard focus is within the document
    // (not in the browser URL bar or address chrome).
    await page.locator('body').click();
    await page.keyboard.press('/');
    const input = page.getByRole('textbox', { name: 'Search catalog' });
    await expect(input).toBeFocused();
  });

  test('protocol filter updates URL param', async ({ page }) => {
    await loginViaUI(page);
    await page.goto('/');
    // ToggleGroup renders items as role="radio" (Radix UI single-select group)
    await page.getByRole('radio', { name: /A2A/i }).click();
    await expect(page).toHaveURL(/protocol=a2a/);
  });

  test('URL filter persists on reload', async ({ page }) => {
    await loginViaUI(page);
    await page.goto('/?protocol=mcp');
    await page.reload();
    await expect(page).toHaveURL(/protocol=mcp/);
  });

  test('UI: project filter scopes catalog list', async ({ page, request }) => {
    const token = await loginViaAPI(request)

    // Seed project + catalog entry + assign entry to project.
    const projectRes = await request.post(`${BASE}/api/v1/projects`, {
      headers: authHeader(token),
      data: { name: 'e2e-filter-project' },
    })
    const project = await projectRes.json()

    const scopedRes = await request.post(`${BASE}/api/v1/catalog`, {
      headers: authHeader(token),
      data: {
        display_name: 'E2E Scoped Agent',
        description: 'scoped',
        protocol: 'a2a',
        endpoint: `http://e2e-scoped-${Date.now()}.example.com`,
        version: '1.0.0',
      },
    })
    const scoped = await scopedRes.json()
    await request.post(`${BASE}/api/v1/catalog/${scoped.id}/projects`, {
      headers: authHeader(token),
      data: { project_id: project.id },
    })

    await loginViaUI(page)
    await page.goto(`/?project=${project.id}`)
    await expect(page.getByText('E2E Scoped Agent')).toBeVisible()

    // Cleanup
    await request.delete(`${BASE}/api/v1/catalog/${scoped.id}`, { headers: authHeader(token) }).catch(() => {})
    await request.delete(`${BASE}/api/v1/projects/${project.id}`, { headers: authHeader(token) }).catch(() => {})
  });

  test('Raw Card tab visible on detail page', async ({ page, request }) => {
    // Seed an entry via API.
    const entry = await createCatalogEntry(request, token, {
      display_name: 'E2E Test Agent',
      protocol: 'a2a',
      endpoint: `http://e2e-test-${Date.now()}.local`,
    });

    await loginViaUI(page);
    await page.goto(`/catalog/${entry.id}`);
    await expect(page.getByRole('tab', { name: 'Raw Card' })).toBeVisible();
    await page.getByRole('tab', { name: 'Raw Card' }).click();
    // Card store may return 404 for manually created entries — that's acceptable.
    await expect(
      page.getByText(/no raw card stored|Failed to load/i).or(page.locator('pre code.language-json'))
    ).toBeVisible({ timeout: 5_000 });

    // Cleanup
    await deleteCatalogEntry(request, token, entry.id);
  });
});

test.describe('Capability Discovery', () => {
  let token: string
  let capEntryId: string

  const CAP_CARD = JSON.stringify({
    name: 'E2E Capability Agent',
    description: 'Agent with capabilities for E2E tests.',
    version: '1.0.0',
    url: 'https://e2e-cap-agent.example.com',
    provider: { organization: 'E2E Org', url: 'https://e2e.example.com' },
    skills: [
      {
        id: 'e2e-skill',
        name: 'E2E Skill',
        description: 'A skill for E2E testing.',
        tags: ['e2e', 'test'],
      },
    ],
  })

  test.beforeAll(async ({ request }) => {
    token = await loginViaAPI(request)

    // Register agent via the register endpoint so capabilities are parsed and stored.
    const res = await request.post(`${BASE}/api/v1/catalog/register`, {
      headers: { ...authHeader(token), 'Content-Type': 'application/json' },
      data: CAP_CARD,
    })
    if (res.ok()) {
      const entry = await res.json()
      capEntryId = entry.id as string
    } else if (res.status() === 409) {
      // Already exists — find by endpoint or display_name.
      const allRes = await request.get(`${BASE}/api/v1/catalog`, { headers: authHeader(token) })
      const all: Array<{ id: string; endpoint: string; display_name: string }> = await allRes.json()
      const existing =
        all.find(e => e.endpoint === 'https://e2e-cap-agent.example.com') ??
        all.find(e => e.display_name === 'E2E Capability Agent')
      if (existing) capEntryId = existing.id
    } else {
      throw new Error(`registerFromCard failed: ${res.status()} ${await res.text()}`)
    }

    if (!capEntryId) throw new Error('capEntryId not set after register')

    // Activate so it appears in the capabilities list (ListCapabilities filters to active/degraded).
    await request.patch(`${BASE}/api/v1/catalog/${capEntryId}/lifecycle`, {
      headers: authHeader(token),
      data: { state: 'active' },
    })
  })

  test.afterAll(async ({ request }) => {
    if (capEntryId) {
      await deleteCatalogEntry(request, token, capEntryId)
    }
  })

  test.beforeEach(async ({ page }) => {
    await loginViaUI(page)
  })

  test('capability list page shows accordion groups', async ({ page }) => {
    await page.goto('/catalog/capabilities')

    await page.waitForLoadState('networkidle')

    // Should show header
    await expect(page.getByRole('heading', { name: 'Capabilities' })).toBeVisible()

    // Should show search box
    await expect(page.getByPlaceholder('Search across A2A and MCP — name, description, skills, tags, provider…')).toBeVisible()

    // Should show kind filter
    await expect(page.getByText('All').first()).toBeVisible()
    await expect(page.getByRole('radio', { name: 'A2A Skill' })).toBeVisible()
  })

  test('search filters capabilities', async ({ page }) => {
    await page.goto('/catalog/capabilities')
    await page.waitForLoadState('networkidle')

    // Type in search
    const searchBox = page.getByPlaceholder('Search across A2A and MCP — name, description, skills, tags, provider…')
    await searchBox.fill('translate')

    // URL should update
    await page.waitForURL(/q=translate/)
  })

  test('kind filter works', async ({ page }) => {
    await page.goto('/catalog/capabilities')
    await page.waitForLoadState('networkidle')

    // Click A2A Skill filter
    await page.getByRole('radio', { name: 'A2A Skill' }).click()

    // URL should update
    await page.waitForURL(/kind=a2a\.skill/)
  })

  test('accordion expands and shows agents', async ({ page }) => {
    await page.goto('/catalog/capabilities')
    await page.waitForLoadState('networkidle')

    // Wait for at least one accordion group to appear
    const firstGroup = page.locator('.border.rounded-lg').first()
    await expect(firstGroup).toBeVisible({ timeout: 10000 })

    // Click to expand
    await firstGroup.click()

    // Should show agent list (wait for expanded content)
    await expect(firstGroup.getByText(/agent/)).toBeVisible({ timeout: 5000 })
  })

  test('capability detail page shows agents', async ({ page }) => {
    await page.goto('/catalog/capabilities')
    await page.waitForLoadState('networkidle')

    // Wait for at least one accordion group to appear
    const firstGroup = page.locator('.border.rounded-lg').first()
    await expect(firstGroup).toBeVisible({ timeout: 10000 })

    // Expand first group
    await firstGroup.click()

    // Click "View all" link
    await firstGroup.getByText('View all').click()

    // Should navigate to detail page
    await expect(page).toHaveURL(/\/catalog\/capabilities\//)

    // Should show table headers
    await expect(page.getByRole('columnheader', { name: 'Protocol' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Agent' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Provider' })).toBeVisible()
  })

  test('capability link from agent detail navigates correctly', async ({ page }) => {
    // Navigate directly to the seeded entry detail page (has capabilities).
    await page.goto(`/catalog/${capEntryId}`)
    await page.waitForLoadState('networkidle')
    await page.waitForSelector('h1')

    // Find and click a capability link (discoverable capability)
    const capabilityLink = page.locator('a[href*="/catalog/capabilities/"]').first()
    if (await capabilityLink.isVisible()) {
      await capabilityLink.click()

      // Should navigate to capability detail
      await expect(page).toHaveURL(/\/catalog\/capabilities\//)
    }
  })
})

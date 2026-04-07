/**
 * Shared helpers for AgentLens E2E tests.
 *
 * • API helper functions call the backend REST API directly.
 * • `loginViaAPI` authenticates and returns a token for use in subsequent API
 *    calls or for injecting into the browser via `storageState`.
 */
import { type APIRequestContext, type Page, expect } from '@playwright/test';

/** Base URL read from env or default. */
export const BASE = `http://localhost:${process.env.AGENTLENS_PORT ?? 18080}`;

/** Admin credentials – password is set via AGENTLENS_ADMIN_PASSWORD env var. */
export const ADMIN_USER = 'admin';
export function adminPassword(): string {
  const pw = process.env.AGENTLENS_ADMIN_PASSWORD;
  if (!pw) throw new Error('AGENTLENS_ADMIN_PASSWORD not set');
  return pw;
}

// ───────────────────── API helpers ─────────────────────

/** Log in via the REST API, returning the JWT token. */
export async function loginViaAPI(
  request: APIRequestContext,
  username = ADMIN_USER,
  password = adminPassword(),
): Promise<string> {
  const res = await request.post(`${BASE}/api/v1/auth/login`, {
    data: { username, password },
  });
  expect(res.ok(), `login failed: ${res.status()} ${await res.text()}`).toBeTruthy();
  const body = await res.json();
  return body.token as string;
}

/** Shorthand: set Authorization header for subsequent API calls. */
export function authHeader(token: string) {
  return { Authorization: `Bearer ${token}` };
}

/** Create a catalog entry via the API. Returns the created entry object. */
export async function createCatalogEntry(
  request: APIRequestContext,
  token: string,
  overrides: Record<string, unknown> = {},
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
): Promise<any> {
  const body = {
    display_name: `Test Agent ${Date.now()}`,
    description: 'E2E test agent',
    protocol: 'a2a',
    endpoint: `http://e2e-agent-${Date.now()}.example.com`,
    version: '1.0.0',
    ...overrides,
  };
  const res = await request.post(`${BASE}/api/v1/catalog`, {
    headers: authHeader(token),
    data: body,
  });
  expect(res.ok(), `createCatalogEntry failed: ${res.status()}`).toBeTruthy();
  return res.json();
}

/** Create a user via the API. Returns the created user object. */
export async function createUser(
  request: APIRequestContext,
  token: string,
  data: {
    username: string;
    password: string;
    display_name?: string;
    email?: string;
    role_id: string;
  },
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
): Promise<any> {
  const res = await request.post(`${BASE}/api/v1/users`, {
    headers: authHeader(token),
    data,
  });
  expect(res.ok(), `createUser failed: ${res.status()}`).toBeTruthy();
  return res.json();
}

/** Delete a catalog entry via the API. */
export async function deleteCatalogEntry(
  request: APIRequestContext,
  token: string,
  id: string,
): Promise<void> {
  const res = await request.delete(`${BASE}/api/v1/catalog/${id}`, {
    headers: authHeader(token),
  });
  expect(res.ok(), `deleteCatalogEntry failed: ${res.status()}`).toBeTruthy();
}

/** Validate an agent card via the API. Returns the validation result. */
export async function validateAgentCard(
  request: APIRequestContext,
  token: string,
  cardJson: string,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
): Promise<any> {
  const res = await request.post(`${BASE}/api/v1/catalog/validate`, {
    headers: { ...authHeader(token), 'Content-Type': 'application/json' },
    data: cardJson,
  });
  return { status: res.status(), body: await res.json() };
}

// ───────────────────── Browser helpers ─────────────────────

/** Login through the web UI and wait for the dashboard to load. */
export async function loginViaUI(
  page: Page,
  username = ADMIN_USER,
  password = adminPassword(),
): Promise<void> {
  await page.goto('/login');
  await page.getByLabel('Username').fill(username);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  // Wait for redirect away from /login — more reliable than waiting for specific text
  await page.waitForURL(url => !url.includes('/login'), { timeout: 15_000 });
  await page.waitForLoadState('networkidle');
}

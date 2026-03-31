import { test, expect } from '@playwright/test';
import { loginViaAPI, createUser, authHeader, BASE } from './helpers';

test.describe('Permission-Based Access Control', () => {
  let adminToken: string;
  let viewerUserId: string;
  const viewerUsername = `viewer-${Date.now()}`;
  const viewerPassword = 'ViewerP@ss123!';

  test.beforeAll(async ({ request }) => {
    adminToken = await loginViaAPI(request);

    // Get viewer role
    const rolesRes = await request.get(`${BASE}/api/v1/roles`, {
      headers: authHeader(adminToken),
    });
    const roles = await rolesRes.json();
    const viewerRole = roles.find((r: { name: string }) => r.name === 'viewer');

    // Create a viewer user
    const user = await createUser(request, adminToken, {
      username: viewerUsername,
      password: viewerPassword,
      role_id: viewerRole.id,
    });
    viewerUserId = user.id;
  });

  test.afterAll(async ({ request }) => {
    // Cleanup viewer user
    if (viewerUserId) {
      await request.delete(`${BASE}/api/v1/users/${viewerUserId}`, {
        headers: authHeader(adminToken),
      });
    }
  });

  test('viewer can read catalog', async ({ request }) => {
    const viewerToken = await loginViaAPI(request, viewerUsername, viewerPassword);

    const res = await request.get(`${BASE}/api/v1/catalog`, {
      headers: authHeader(viewerToken),
    });
    expect(res.ok()).toBeTruthy();
  });

  test('viewer cannot create catalog entries', async ({ request }) => {
    const viewerToken = await loginViaAPI(request, viewerUsername, viewerPassword);

    const res = await request.post(`${BASE}/api/v1/catalog`, {
      headers: authHeader(viewerToken),
      data: {
        display_name: 'Unauthorized Entry',
        protocol: 'a2a',
        endpoint: `http://unauthorized-${Date.now()}.example.com`,
      },
    });
    expect(res.status()).toBe(403);
  });

  test('viewer cannot delete catalog entries', async ({ request }) => {
    // Create an entry as admin
    const entry = await request.post(`${BASE}/api/v1/catalog`, {
      headers: authHeader(adminToken),
      data: {
        display_name: 'Perm Test Entry',
        protocol: 'a2a',
        endpoint: `http://perm-test-${Date.now()}.example.com`,
      },
    });
    const entryData = await entry.json();

    const viewerToken = await loginViaAPI(request, viewerUsername, viewerPassword);

    const deleteRes = await request.delete(`${BASE}/api/v1/catalog/${entryData.id}`, {
      headers: authHeader(viewerToken),
    });
    expect(deleteRes.status()).toBe(403);

    // Admin cleanup
    await request.delete(`${BASE}/api/v1/catalog/${entryData.id}`, {
      headers: authHeader(adminToken),
    });
  });

  test('viewer cannot manage users', async ({ request }) => {
    const viewerToken = await loginViaAPI(request, viewerUsername, viewerPassword);

    const res = await request.post(`${BASE}/api/v1/users`, {
      headers: authHeader(viewerToken),
      data: {
        username: 'hacker',
        password: 'H@ckPass123!',
        role_id: 'role-admin',
      },
    });
    expect(res.status()).toBe(403);
  });

  test('viewer cannot modify roles', async ({ request }) => {
    const viewerToken = await loginViaAPI(request, viewerUsername, viewerPassword);

    const res = await request.post(`${BASE}/api/v1/roles`, {
      headers: authHeader(viewerToken),
      data: {
        name: 'hacked-role',
        permissions: ['catalog:read', 'catalog:write', 'catalog:delete'],
      },
    });
    expect(res.status()).toBe(403);
  });

  test('viewer cannot update settings', async ({ request }) => {
    const viewerToken = await loginViaAPI(request, viewerUsername, viewerPassword);

    const res = await request.put(`${BASE}/api/v1/settings`, {
      headers: authHeader(viewerToken),
      data: { 'ui.theme': 'dark' },
    });
    expect(res.status()).toBe(403);
  });
});

import { test, expect } from '@playwright/test';
import { loginViaAPI, loginViaUI, createUser, authHeader, BASE, adminPassword } from './helpers';

test.describe('User Management', () => {
  let adminToken: string;

  test.beforeAll(async ({ request }) => {
    adminToken = await loginViaAPI(request);
  });

  test('API: list users returns admin user', async ({ request }) => {
    const res = await request.get(`${BASE}/api/v1/users`, {
      headers: authHeader(adminToken),
    });
    expect(res.ok()).toBeTruthy();
    const users = await res.json();
    expect(Array.isArray(users)).toBeTruthy();
    expect(users.some((u: { username: string }) => u.username === 'admin')).toBeTruthy();
  });

  test('API: create, get, update, and delete a user', async ({ request }) => {
    const username = `testuser-${Date.now()}`;

    // Get role list to find a valid role ID
    const rolesRes = await request.get(`${BASE}/api/v1/roles`, {
      headers: authHeader(adminToken),
    });
    const roles = await rolesRes.json();
    const viewerRole = roles.find((r: { name: string }) => r.name === 'viewer');
    expect(viewerRole, 'viewer role should exist').toBeTruthy();

    // Create user
    const user = await createUser(request, adminToken, {
      username,
      password: 'TestP@ss123!',
      display_name: 'Test User',
      email: `${username}@test.com`,
      role_id: viewerRole.id,
    });
    expect(user.id).toBeTruthy();
    expect(user.username).toBe(username);

    // Get user by ID
    const getRes = await request.get(`${BASE}/api/v1/users/${user.id}`, {
      headers: authHeader(adminToken),
    });
    expect(getRes.ok()).toBeTruthy();
    const fetched = await getRes.json();
    expect(fetched.username).toBe(username);

    // Update user
    const updateRes = await request.put(`${BASE}/api/v1/users/${user.id}`, {
      headers: authHeader(adminToken),
      data: { display_name: 'Updated Test User' },
    });
    expect(updateRes.ok()).toBeTruthy();

    // Verify update
    const verifyRes = await request.get(`${BASE}/api/v1/users/${user.id}`, {
      headers: authHeader(adminToken),
    });
    const updated = await verifyRes.json();
    expect(updated.display_name).toBe('Updated Test User');

    // Delete user
    const deleteRes = await request.delete(`${BASE}/api/v1/users/${user.id}`, {
      headers: authHeader(adminToken),
    });
    expect(deleteRes.ok()).toBeTruthy();

    // Verify deleted
    const verifyDeleteRes = await request.get(`${BASE}/api/v1/users/${user.id}`, {
      headers: authHeader(adminToken),
    });
    expect(verifyDeleteRes.status()).toBe(404);
  });

  test('API: cannot delete self', async ({ request }) => {
    // Get current user ID
    const meRes = await request.get(`${BASE}/api/v1/auth/me`, {
      headers: authHeader(adminToken),
    });
    const me = await meRes.json();

    const delRes = await request.delete(`${BASE}/api/v1/users/${me.id}`, {
      headers: authHeader(adminToken),
    });
    // Should fail (400 or 403)
    expect(delRes.ok()).toBeFalsy();
  });

  test('API: created user can login', async ({ request }) => {
    const username = `loginuser-${Date.now()}`;
    const password = 'Secure123!Pass';

    const rolesRes = await request.get(`${BASE}/api/v1/roles`, {
      headers: authHeader(adminToken),
    });
    const roles = await rolesRes.json();
    const viewerRole = roles.find((r: { name: string }) => r.name === 'viewer');

    const user = await createUser(request, adminToken, {
      username,
      password,
      role_id: viewerRole.id,
    });

    // Login as new user
    const loginRes = await request.post(`${BASE}/api/v1/auth/login`, {
      data: { username, password },
    });
    expect(loginRes.ok()).toBeTruthy();
    const body = await loginRes.json();
    expect(body.token).toBeTruthy();

    // Cleanup
    await request.delete(`${BASE}/api/v1/users/${user.id}`, {
      headers: authHeader(adminToken),
    });
  });

  test('UI: Settings Users tab shows user list', async ({ page }) => {
    await loginViaUI(page);
    await page.goto('/settings');

    // Click on the Users tab
    await page.getByRole('tab', { name: /Users/i }).click();

    // Should see the admin user in the list
    await expect(page.getByText('admin').first()).toBeVisible();
  });
});

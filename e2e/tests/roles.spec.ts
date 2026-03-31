import { test, expect } from '@playwright/test';
import { loginViaAPI, loginViaUI, authHeader, BASE } from './helpers';

test.describe('Role Management', () => {
  let adminToken: string;

  test.beforeAll(async ({ request }) => {
    adminToken = await loginViaAPI(request);
  });

  test('API: list roles returns default roles', async ({ request }) => {
    const res = await request.get(`${BASE}/api/v1/roles`, {
      headers: authHeader(adminToken),
    });
    expect(res.ok()).toBeTruthy();
    const roles = await res.json();
    expect(Array.isArray(roles)).toBeTruthy();

    // Default system roles should exist
    const roleNames = roles.map((r: { name: string }) => r.name);
    expect(roleNames).toContain('admin');
    expect(roleNames).toContain('editor');
    expect(roleNames).toContain('viewer');
  });

  test('API: create, update, and delete a custom role', async ({ request }) => {
    const roleName = `custom-role-${Date.now()}`;

    // Create role
    const createRes = await request.post(`${BASE}/api/v1/roles`, {
      headers: authHeader(adminToken),
      data: {
        name: roleName,
        description: 'E2E test role',
        permissions: ['catalog:read', 'settings:read'],
      },
    });
    expect(createRes.ok()).toBeTruthy();
    const role = await createRes.json();
    expect(role.id).toBeTruthy();
    expect(role.name).toBe(roleName);

    // Update role
    const updateRes = await request.put(`${BASE}/api/v1/roles/${role.id}`, {
      headers: authHeader(adminToken),
      data: {
        name: roleName,
        description: 'Updated E2E test role',
        permissions: ['catalog:read', 'catalog:write', 'settings:read'],
      },
    });
    expect(updateRes.ok()).toBeTruthy();

    // Delete role
    const deleteRes = await request.delete(`${BASE}/api/v1/roles/${role.id}`, {
      headers: authHeader(adminToken),
    });
    expect(deleteRes.ok()).toBeTruthy();
  });

  test('API: cannot delete system roles', async ({ request }) => {
    const res = await request.get(`${BASE}/api/v1/roles`, {
      headers: authHeader(adminToken),
    });
    const roles = await res.json();
    const adminRole = roles.find((r: { name: string }) => r.name === 'admin');
    expect(adminRole).toBeTruthy();

    const delRes = await request.delete(`${BASE}/api/v1/roles/${adminRole.id}`, {
      headers: authHeader(adminToken),
    });
    // System roles cannot be deleted
    expect(delRes.ok()).toBeFalsy();
  });

  test('UI: Settings Roles tab shows system roles', async ({ page }) => {
    await loginViaUI(page);
    await page.goto('/settings');

    // Click on the Roles tab
    await page.getByRole('tab', { name: /Roles/i }).click();

    // Should see the default roles
    await expect(page.getByText('admin').first()).toBeVisible();
    await expect(page.getByText('editor').first()).toBeVisible();
    await expect(page.getByText('viewer').first()).toBeVisible();
  });
});

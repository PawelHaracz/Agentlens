import { test, expect } from '@playwright/test';
import { loginViaAPI, loginViaUI, authHeader, BASE } from './helpers';

test.describe('Settings Management', () => {
  let adminToken: string;

  test.beforeAll(async ({ request }) => {
    adminToken = await loginViaAPI(request);
  });

  test('API: get all settings', async ({ request }) => {
    const res = await request.get(`${BASE}/api/v1/settings`, {
      headers: authHeader(adminToken),
    });
    expect(res.ok()).toBeTruthy();
    const settings = await res.json();
    expect(Array.isArray(settings)).toBeTruthy();
  });

  test('API: update settings', async ({ request }) => {
    // First get current settings
    const getRes = await request.get(`${BASE}/api/v1/settings`, {
      headers: authHeader(adminToken),
    });
    const settings = await getRes.json();

    // Update a setting
    const updateRes = await request.put(`${BASE}/api/v1/settings`, {
      headers: authHeader(adminToken),
      data: { 'ui.theme': 'dark' },
    });
    expect(updateRes.ok()).toBeTruthy();

    // Revert
    await request.put(`${BASE}/api/v1/settings`, {
      headers: authHeader(adminToken),
      data: { 'ui.theme': 'system' },
    });
  });

  test('UI: settings page General tab is accessible', async ({ page }) => {
    await loginViaUI(page);
    // Navigate to settings
    await page.goto('/settings');

    // General tab should be visible/selected by default
    await expect(page.getByRole('tab', { name: /General/i })).toBeVisible();

    // Theme section should be visible
    await expect(page.getByText(/Appearance|Theme/i).first()).toBeVisible();
  });

  test('UI: settings page has all tabs', async ({ page }) => {
    await loginViaUI(page);
    await page.goto('/settings');

    // All 4 tabs should be present
    await expect(page.getByRole('tab', { name: /General/i })).toBeVisible();
    await expect(page.getByRole('tab', { name: /Users/i })).toBeVisible();
    await expect(page.getByRole('tab', { name: /Roles/i })).toBeVisible();
    await expect(page.getByRole('tab', { name: /My Account/i })).toBeVisible();
  });

  test('UI: My Account tab shows profile form', async ({ page }) => {
    await loginViaUI(page);
    await page.goto('/settings');

    // Click My Account tab
    await page.getByRole('tab', { name: /My Account/i }).click();

    // Profile form fields
    await expect(page.getByText(/Profile|Display name|Username/i).first()).toBeVisible();

    // Change password section
    await expect(page.getByText(/Change password|Current password/i).first()).toBeVisible();
  });
});

import { test, expect } from '@playwright/test';
import { loginViaUI, loginViaAPI, adminPassword, ADMIN_USER, BASE, authHeader } from './helpers';

test.describe('Authentication', () => {
  test('shows login page when not authenticated', async ({ page }) => {
    await page.goto('/');
    // Should redirect to login
    await expect(page).toHaveURL(/\/login/);
    await expect(page.getByRole('heading', { name: 'AgentLens' })).toBeVisible();
    await expect(page.getByLabel('Username')).toBeVisible();
    await expect(page.getByLabel('Password')).toBeVisible();
  });

  test('login with valid credentials', async ({ page }) => {
    await loginViaUI(page);
    // Should be on the dashboard
    await expect(page).not.toHaveURL(/\/login/);
  });

  test('login with invalid credentials shows error', async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Username').fill('admin');
    await page.getByLabel('Password').fill('wrong-password');
    await page.getByRole('button', { name: 'Sign in' }).click();
    // The API returns 401 → the request helper throws "Session expired"
    await expect(page.getByText(/Session expired|invalid|failed|Login failed/i)).toBeVisible();
  });

  test('API: login returns JWT token', async ({ request }) => {
    const token = await loginViaAPI(request);
    expect(token).toBeTruthy();
    expect(token.split('.')).toHaveLength(3); // JWT format
  });

  test('API: /auth/me returns current user', async ({ request }) => {
    const token = await loginViaAPI(request);
    const res = await request.get(`${BASE}/api/v1/auth/me`, {
      headers: authHeader(token),
    });
    expect(res.ok()).toBeTruthy();
    const me = await res.json();
    expect(me.username).toBe(ADMIN_USER);
  });

  test('API: refresh token', async ({ request }) => {
    const token = await loginViaAPI(request);
    const res = await request.post(`${BASE}/api/v1/auth/refresh`, {
      headers: authHeader(token),
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.token).toBeTruthy();
  });

  test('API: change password and login with new password', async ({ request }) => {
    const oldPassword = adminPassword();
    const newPassword = 'NewP@ssw0rd!2026';

    const token = await loginViaAPI(request);

    // Change password
    const changeRes = await request.put(`${BASE}/api/v1/auth/password`, {
      headers: authHeader(token),
      data: { current_password: oldPassword, new_password: newPassword },
    });
    expect(changeRes.ok(), `change password failed: ${changeRes.status()}`).toBeTruthy();

    // Login with new password
    const newToken = await loginViaAPI(request, ADMIN_USER, newPassword);
    expect(newToken).toBeTruthy();

    // Revert password back to original
    const revertRes = await request.put(`${BASE}/api/v1/auth/password`, {
      headers: authHeader(newToken),
      data: { current_password: newPassword, new_password: oldPassword },
    });
    expect(revertRes.ok(), `revert password failed: ${revertRes.status()}`).toBeTruthy();
  });

  test('API: unauthenticated request returns 401', async ({ request }) => {
    const res = await request.get(`${BASE}/api/v1/catalog`);
    expect(res.status()).toBe(401);
  });

  test('API: logout', async ({ request }) => {
    const token = await loginViaAPI(request);
    const res = await request.post(`${BASE}/api/v1/auth/logout`, {
      headers: authHeader(token),
    });
    expect(res.ok()).toBeTruthy();
  });

  test('user dropdown menu is visible after login', async ({ page }) => {
    await loginViaUI(page);
    // Click on the user avatar / initials button
    const userButton = page.locator('button').filter({ hasText: /^A$/ }).or(
      page.getByRole('button', { name: /Administrator|admin/i })
    );
    await userButton.first().click();
    // The dropdown should show user-related menu items
    await expect(page.getByText(/My Account|Settings|Logout/i).first()).toBeVisible();
  });
});

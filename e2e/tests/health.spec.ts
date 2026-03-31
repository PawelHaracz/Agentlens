import { test, expect } from '@playwright/test';
import { BASE } from './helpers';

test.describe('Health Check', () => {
  test('GET /healthz returns 200', async ({ request }) => {
    const res = await request.get(`${BASE}/healthz`);
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.status).toBe('ok');
  });
});

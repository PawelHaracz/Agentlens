import { test, expect } from '@playwright/test'

test.describe('OTel observability endpoints', () => {
  test('GET /healthz returns ok', async ({ request }) => {
    const resp = await request.get('/healthz')
    expect(resp.ok()).toBeTruthy()
    const body = await resp.json()
    expect(body.status).toBe('ok')
  })

  test('GET /readyz returns ok when DB is healthy', async ({ request }) => {
    const resp = await request.get('/readyz')
    // Should be 200 in a running test environment
    expect(resp.status()).toBe(200)
    const body = await resp.json()
    expect(body.status).toBe('ok')
  })

  test('GET /api/v1/telemetry/config returns expected shape', async ({ request }) => {
    const resp = await request.get('/api/v1/telemetry/config')
    expect(resp.ok()).toBeTruthy()
    const body = await resp.json()
    // Must have enabled field (true or false)
    expect(typeof body.enabled).toBe('boolean')
    // When disabled (test environment), no endpoint or serviceName
    if (!body.enabled) {
      expect(body.endpoint ?? '').toBe('')
    }
  })
})

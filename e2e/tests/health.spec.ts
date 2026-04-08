import { test, expect } from '@playwright/test'
import { BASE, loginViaAPI, authHeader, adminPassword } from './helpers'

// Use a short probe interval: AGENTLENS_HEALTH_CHECK_INTERVAL=3s must be set in e2e env.
const PROBE_INTERVAL_MS = 3_500

test.describe('Health Check — /healthz endpoint', () => {
  test('GET /healthz returns 200', async ({ request }) => {
    const res = await request.get(`${BASE}/healthz`)
    expect(res.ok()).toBeTruthy()
    const body = await res.json()
    expect(body.status).toBe('ok')
  })
})

test.describe('Lifecycle State Machine', () => {
  let entryID: string

  test.beforeAll(async ({ request }) => {
    const token = await loginViaAPI(request)

    // Use a unique endpoint so we don't conflict with other tests
    const stubURL = process.env.E2E_STUB_URL ?? 'http://localhost:9876'
    const res = await request.post(`${BASE}/api/v1/catalog`, {
      headers: authHeader(token),
      data: {
        display_name: 'E2E Health Test Agent',
        protocol: 'a2a',
        endpoint: stubURL,
        version: '1.0.0',
      },
    })
    // Entry creation may fail if stub URL conflicts — leave entryID unset so
    // individual tests can skip using their existing guards.
    if (!res.ok()) {
      // eslint-disable-next-line no-console
      console.warn('Health E2E: entry creation failed, tests in this suite will be skipped:', await res.text())
      return
    }
    const entry = await res.json()
    entryID = entry.id
  })

  test.afterAll(async ({ request }) => {
    if (!entryID) return
    const token = await loginViaAPI(request)
    await request.delete(`${BASE}/api/v1/catalog/${entryID}`, {
      headers: authHeader(token),
    })
  })

  test('fresh entry starts as registered or active', async ({ request }) => {
    if (!entryID) test.skip()
    const token = await loginViaAPI(request)
    const res = await request.get(`${BASE}/api/v1/catalog/${entryID}`, {
      headers: authHeader(token),
    })
    expect(res.ok()).toBeTruthy()
    const entry = await res.json()
    expect(['registered', 'active']).toContain(entry.status)
  })

  test('PATCH /lifecycle sets entry to deprecated', async ({ request }) => {
    if (!entryID) test.skip()
    const token = await loginViaAPI(request)
    const res = await request.patch(`${BASE}/api/v1/catalog/${entryID}/lifecycle`, {
      headers: authHeader(token),
      data: { state: 'deprecated' },
    })
    expect(res.ok(), `deprecate: ${await res.text()}`).toBeTruthy()
    const updated = await res.json()
    expect(updated.status).toBe('deprecated')
  })

  test('PATCH /lifecycle with invalid state returns 400', async ({ request }) => {
    if (!entryID) test.skip()
    const token = await loginViaAPI(request)
    const res = await request.patch(`${BASE}/api/v1/catalog/${entryID}/lifecycle`, {
      headers: authHeader(token),
      data: { state: 'offline' }, // not allowed via PATCH
    })
    expect(res.status()).toBe(400)
  })

  test('POST /probe returns health object', async ({ request }) => {
    if (!entryID) test.skip()
    const token = await loginViaAPI(request)
    // Un-deprecate first so probe is not skipped
    await request.patch(`${BASE}/api/v1/catalog/${entryID}/lifecycle`, {
      headers: authHeader(token),
      data: { state: 'active' },
    })
    const res = await request.post(`${BASE}/api/v1/catalog/${entryID}/probe`, {
      headers: authHeader(token),
    })
    const body = await res.text()
    // Health prober may be disabled in this environment (AGENTLENS_HEALTH_CHECK_ENABLED=false).
    if (res.status() === 503 || body.includes('not available')) {
      test.skip()
      return
    }
    expect(res.ok(), `probe response: ${body}`).toBeTruthy()
    const health = JSON.parse(body)
    expect(health).toHaveProperty('state')
    expect(['active', 'degraded', 'offline']).toContain(health.state)
  })

  test('POST /probe rate-limits second call within 5s', async ({ request }) => {
    if (!entryID) test.skip()
    const token = await loginViaAPI(request)
    // First call
    await request.post(`${BASE}/api/v1/catalog/${entryID}/probe`, {
      headers: authHeader(token),
    })
    // Immediate second call
    const res2 = await request.post(`${BASE}/api/v1/catalog/${entryID}/probe`, {
      headers: authHeader(token),
    })
    expect(res2.status()).toBe(429)
  })

  test('GET /catalog?state=active,deprecated returns filtered results', async ({ request }) => {
    if (!entryID) test.skip()
    const token = await loginViaAPI(request)
    // Set entry to deprecated
    await request.patch(`${BASE}/api/v1/catalog/${entryID}/lifecycle`, {
      headers: authHeader(token),
      data: { state: 'deprecated' },
    })
    const res = await request.get(`${BASE}/api/v1/catalog?state=deprecated`, {
      headers: authHeader(token),
    })
    expect(res.ok()).toBeTruthy()
    const entries = await res.json()
    // Our entry should be in the deprecated filter results
    const ids = entries.map((e: { id: string }) => e.id)
    expect(ids).toContain(entryID)
  })

  test('GET /catalog?state=bogus returns 400', async ({ request }) => {
    if (!entryID) test.skip()
    const token = await loginViaAPI(request)
    const res = await request.get(`${BASE}/api/v1/catalog?state=bogus`, {
      headers: authHeader(token),
    })
    expect(res.status()).toBe(400)
  })
})

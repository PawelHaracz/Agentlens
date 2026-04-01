import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { validateAgentCard, createAgentFromCard, setToken } from './api'
import type { ValidationResult, CatalogEntry } from './types'

const originalFetch = globalThis.fetch

beforeEach(() => {
  setToken('test-token')
})

afterEach(() => {
  globalThis.fetch = originalFetch
  setToken(null)
})

describe('validateAgentCard', () => {
  it('returns ValidationResult for a valid card (200)', async () => {
    const mockResult: ValidationResult = {
      valid: true,
      spec_version: '0.2.1',
      errors: [],
      warnings: [],
      preview: {
        display_name: 'Test Agent',
        description: 'A test agent',
        protocol: 'a2a',
        spec_version: '0.2.1',
        skills_count: 2,
        extensions_count: 0,
        security_schemes: [],
        interfaces: [],
      },
    }

    globalThis.fetch = vi.fn().mockResolvedValue({
      status: 200,
      ok: true,
      json: () => Promise.resolve(mockResult),
    })

    const result = await validateAgentCard('{"name":"Test Agent"}')

    expect(result).toEqual(mockResult)
    expect(result.valid).toBe(true)
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/v1/catalog/validate',
      expect.objectContaining({
        method: 'POST',
        body: '{"name":"Test Agent"}',
      }),
    )
  })

  it('returns ValidationResult for an invalid card (422)', async () => {
    const mockResult: ValidationResult = {
      valid: false,
      spec_version: '',
      errors: [{ field: 'name', message: 'name is required' }],
      warnings: [],
    }

    globalThis.fetch = vi.fn().mockResolvedValue({
      status: 422,
      ok: false,
      json: () => Promise.resolve(mockResult),
    })

    const result = await validateAgentCard('{}')

    expect(result).toEqual(mockResult)
    expect(result.valid).toBe(false)
    expect(result.errors).toHaveLength(1)
    expect(result.errors[0].field).toBe('name')
  })

  it('throws on server error (500)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      status: 500,
      ok: false,
      json: () => Promise.resolve({ error: 'Internal Server Error' }),
    })

    await expect(validateAgentCard('{"name":"Test"}')).rejects.toThrow(
      'Internal Server Error',
    )
  })

  it('includes Authorization header when token is set', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      status: 200,
      ok: true,
      json: () => Promise.resolve({ valid: true, spec_version: '', errors: [], warnings: [] }),
    })

    await validateAgentCard('{}')

    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/v1/catalog/validate',
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: 'Bearer test-token',
        }),
      }),
    )
  })
})

describe('createAgentFromCard', () => {
  it('calls POST /api/v1/catalog/register with correct body and auth header', async () => {
    const mockEntry: CatalogEntry = {
      id: 'abc-123',
      display_name: 'Test Agent',
      description: 'A test agent',
      protocol: 'a2a',
      endpoint: 'https://example.com/agent',
      version: '1.0.0',
      status: 'healthy',
      source: 'push',
      validity: { last_seen: '2026-04-01T00:00:00Z' },
      created_at: '2026-04-01T00:00:00Z',
      updated_at: '2026-04-01T00:00:00Z',
    }

    globalThis.fetch = vi.fn().mockResolvedValue({
      status: 201,
      ok: true,
      json: () => Promise.resolve(mockEntry),
    })

    const cardJson = '{"name":"Test Agent"}'
    const result = await createAgentFromCard(cardJson)

    expect(result).toEqual(mockEntry)
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/v1/catalog/register',
      expect.objectContaining({
        method: 'POST',
        body: cardJson,
        headers: expect.objectContaining({
          Authorization: 'Bearer test-token',
          'Content-Type': 'application/json',
        }),
      }),
    )
  })

  it('throws on error response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      status: 400,
      ok: false,
      json: () => Promise.resolve({ error: 'Bad Request' }),
    })

    await expect(createAgentFromCard('{}')).rejects.toThrow('Bad Request')
  })
})

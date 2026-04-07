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
      status: 'active',
      source: 'push',
      agent_type_id: 'a2a-agent',
      validity: { last_seen: '2026-04-01T00:00:00Z' },
      health: {
        state: 'active',
        latencyMs: 0,
        consecutiveFailures: 0,
        lastError: '',
        lastProbedAt: '2026-04-01T00:00:00Z',
        lastSuccessAt: '2026-04-01T00:00:00Z',
      },
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

import {
  listCatalog, getEntry, deleteEntry, getStats,
  importCardFromURL, login, logout, getMe, refreshToken,
  changePassword, listUsers, createUser, getUser,
  updateUser, deleteUser, listRoles, createRole,
  updateRole, deleteRole, getSettings, getSettingsByCategory,
  updateSettings,
} from './api'

const mockEntry = {
  id: 'e1', display_name: 'Agent', description: '', protocol: 'a2a' as const,
  endpoint: 'https://x.com', version: '1.0', status: 'active' as const,
  source: 'push' as const, agent_type_id: 't1',
  validity: { last_seen: '2026-01-01T00:00:00Z' },
  health: { state: 'active' as const, latencyMs: 0, consecutiveFailures: 0, lastError: '' },
  created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
}

function mockOk(data: unknown, status = 200) {
  return {
    status,
    ok: true,
    json: () => Promise.resolve(data),
  }
}

function mockErr(data: unknown, status = 400) {
  return {
    status,
    ok: false,
    json: () => Promise.resolve(data),
  }
}

function mockNoContent() {
  return { status: 204, ok: true, json: () => Promise.resolve(null) }
}

describe('listCatalog', () => {
  it('returns catalog entries', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk([mockEntry]))
    const result = await listCatalog()
    expect(result).toEqual([mockEntry])
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/catalog', expect.anything())
  })

  it('appends filter query params', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk([]))
    await listCatalog({ protocol: 'a2a', status: 'active', q: 'bot', limit: 10, offset: 5 })
    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringContaining('protocol=a2a'),
      expect.anything(),
    )
  })

  it('throws on error response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockErr({ error: 'Server error' }))
    await expect(listCatalog()).rejects.toThrow('Server error')
  })
})

describe('getEntry', () => {
  it('returns a catalog entry by id', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(mockEntry))
    const result = await getEntry('e1')
    expect(result).toEqual(mockEntry)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/catalog/e1', expect.anything())
  })
})

describe('deleteEntry', () => {
  it('sends DELETE request and returns void on 204', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockNoContent())
    const result = await deleteEntry('e1')
    expect(result).toBeUndefined()
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/catalog/e1', expect.objectContaining({ method: 'DELETE' }))
  })
})

describe('getStats', () => {
  it('returns stats object', async () => {
    const stats = { total: 5, by_status: { healthy: 5 }, by_source: { push: 5 } }
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(stats))
    const result = await getStats()
    expect(result).toEqual(stats)
  })
})

describe('importCardFromURL', () => {
  it('calls POST /api/v1/catalog/import', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(mockEntry, 201))
    const result = await importCardFromURL({ url: 'https://example.com/card.json' })
    expect(result).toEqual(mockEntry)
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/v1/catalog/import',
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('includes protocol when specified', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(mockEntry, 201))
    await importCardFromURL({ url: 'https://example.com/card.json', protocol: 'a2a' })
    const [, init] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0]
    const body = JSON.parse(init.body)
    expect(body.protocol).toBe('a2a')
  })

  it('throws on error', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockErr({ error: 'Not found' }))
    await expect(importCardFromURL({ url: 'https://bad.com' })).rejects.toThrow('Not found')
  })
})

describe('login', () => {
  it('calls POST /api/v1/auth/login and returns token and user', async () => {
    const resp = { token: 'tok', user: { id: '1', username: 'alice' } }
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(resp))
    const result = await login('alice', 'pass')
    expect(result).toEqual(resp)
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/v1/auth/login',
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('throws on invalid credentials', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockErr({ error: 'Unauthorized' }, 401))
    await expect(login('alice', 'wrong')).rejects.toThrow()
  })
})

describe('logout', () => {
  it('calls POST /api/v1/auth/logout', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockNoContent())
    await logout()
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/auth/logout', expect.objectContaining({ method: 'POST' }))
  })
})

describe('getMe', () => {
  it('returns the current user', async () => {
    const user = { id: '1', username: 'alice' }
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(user))
    const result = await getMe()
    expect(result).toEqual(user)
  })
})

describe('refreshToken', () => {
  it('calls POST /api/v1/auth/refresh and returns token', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk({ token: 'new-token' }))
    const result = await refreshToken()
    expect(result).toEqual({ token: 'new-token' })
  })
})

describe('changePassword', () => {
  it('calls PUT /api/v1/auth/password', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockNoContent())
    await changePassword('old', 'new')
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/v1/auth/password',
      expect.objectContaining({ method: 'PUT' }),
    )
  })
})

describe('listUsers', () => {
  it('returns list of users', async () => {
    const users = [{ id: '1', username: 'alice' }]
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(users))
    const result = await listUsers()
    expect(result).toEqual(users)
  })
})

describe('createUser', () => {
  it('calls POST /api/v1/users', async () => {
    const user = { id: '2', username: 'bob' }
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(user, 201))
    const result = await createUser({ username: 'bob', password: 'pass', role_id: 'r1' })
    expect(result).toEqual(user)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/users', expect.objectContaining({ method: 'POST' }))
  })
})

describe('getUser', () => {
  it('returns user by id', async () => {
    const user = { id: '1', username: 'alice' }
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(user))
    const result = await getUser('1')
    expect(result).toEqual(user)
  })
})

describe('updateUser', () => {
  it('calls PUT /api/v1/users/:id', async () => {
    const user = { id: '1', username: 'alice', display_name: 'Alice' }
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(user))
    const result = await updateUser('1', { display_name: 'Alice' })
    expect(result).toEqual(user)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/users/1', expect.objectContaining({ method: 'PUT' }))
  })
})

describe('deleteUser', () => {
  it('calls DELETE /api/v1/users/:id', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockNoContent())
    await deleteUser('1')
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/users/1', expect.objectContaining({ method: 'DELETE' }))
  })
})

describe('listRoles', () => {
  it('returns list of roles', async () => {
    const roles = [{ id: 'r1', name: 'admin', permissions: [] }]
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(roles))
    const result = await listRoles()
    expect(result).toEqual(roles)
  })
})

describe('createRole', () => {
  it('calls POST /api/v1/roles', async () => {
    const role = { id: 'r2', name: 'editor', permissions: ['catalog:read'] }
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(role, 201))
    const result = await createRole({ name: 'editor', permissions: ['catalog:read'] })
    expect(result).toEqual(role)
  })
})

describe('updateRole', () => {
  it('calls PUT /api/v1/roles/:id', async () => {
    const role = { id: 'r1', name: 'admin', permissions: [] }
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(role))
    const result = await updateRole('r1', { name: 'admin' })
    expect(result).toEqual(role)
  })
})

describe('deleteRole', () => {
  it('calls DELETE /api/v1/roles/:id', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockNoContent())
    await deleteRole('r1')
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/roles/r1', expect.objectContaining({ method: 'DELETE' }))
  })
})

describe('getSettings', () => {
  it('returns list of settings', async () => {
    const settings = [{ key: 'ui.theme', value: 'dark', category: 'ui', description: '' }]
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk(settings))
    const result = await getSettings()
    expect(result).toEqual(settings)
  })
})

describe('getSettingsByCategory', () => {
  it('calls GET /api/v1/settings/:category', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockOk([]))
    await getSettingsByCategory('ui')
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/settings/ui', expect.anything())
  })
})

describe('updateSettings', () => {
  it('calls PUT /api/v1/settings', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(mockNoContent())
    await updateSettings({ 'ui.theme': 'dark' })
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/settings', expect.objectContaining({ method: 'PUT' }))
  })
})

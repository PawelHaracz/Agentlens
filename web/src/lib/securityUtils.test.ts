import { describe, it, expect } from 'vitest'
import { buildAuthSummaryLabel, generateCurlRecipe, formatScopesLabel } from './securityUtils'

describe('buildAuthSummaryLabel', () => {
  it('returns "Open (no auth)" for empty types', () => {
    expect(buildAuthSummaryLabel([])).toBe('Open (no auth)')
  })

  it('returns "Bearer JWT" for http:Bearer', () => {
    expect(buildAuthSummaryLabel(['http:Bearer'])).toBe('Bearer JWT')
  })

  it('joins multiple types with " + "', () => {
    expect(buildAuthSummaryLabel(['http:Bearer', 'apiKey'])).toBe('Bearer JWT + API Key')
  })

  it('truncates long labels', () => {
    const label = buildAuthSummaryLabel(['http:Bearer', 'apiKey', 'oauth2', 'openIdConnect', 'mutualTls'])
    expect(label.length).toBeLessThanOrEqual(40)
    expect(label).toContain('...')
  })
})

describe('generateCurlRecipe', () => {
  it('generates Bearer token curl', () => {
    const curl = generateCurlRecipe(
      'https://agent.example.com/api',
      [{ schemes: { httpAuth: [] } }],
      [{ type: 'http', scheme_name: 'httpAuth', http_scheme: 'Bearer' }]
    )
    expect(curl).toContain('curl')
    expect(curl).toContain('-H "Authorization: Bearer <token>"')
    expect(curl).toContain('https://agent.example.com/api')
  })

  it('generates API Key curl', () => {
    const curl = generateCurlRecipe(
      'https://agent.example.com/api',
      [{ schemes: { apiKeyAuth: [] } }],
      [{ type: 'apiKey', scheme_name: 'apiKeyAuth', api_key_location: 'header', api_key_name: 'X-API-Key' }]
    )
    expect(curl).toContain('-H "X-API-Key: <key>"')
  })
})

describe('formatScopesLabel', () => {
  it('returns empty string for no scopes', () => {
    expect(formatScopesLabel([])).toBe('')
  })

  it('formats single scope', () => {
    expect(formatScopesLabel(['read'])).toBe('read')
  })

  it('joins multiple scopes', () => {
    expect(formatScopesLabel(['read', 'write'])).toBe('read, write')
  })
})

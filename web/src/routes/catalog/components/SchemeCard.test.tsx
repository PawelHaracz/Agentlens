import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SchemeCard } from './SchemeCard'
import type { SecurityScheme } from '@/lib/securityUtils'

describe('SchemeCard', () => {
  it('renders HTTP Bearer scheme', () => {
    const scheme: SecurityScheme = {
      scheme_name: 'httpAuth',
      type: 'http',
      http_scheme: 'Bearer',
      bearer_format: 'JWT',
      description: 'JWT authentication',
    }
    const { container } = render(<SchemeCard scheme={scheme} />)
    expect(container).toMatchSnapshot()
  })

  it('renders bearer message conditioned on JWT format', () => {
    const scheme: SecurityScheme = {
      scheme_name: 'httpAuth',
      type: 'http',
      http_scheme: 'Bearer',
      bearer_format: 'JWT',
    }
    render(<SchemeCard scheme={scheme} />)
    expect(screen.getByText(/Expects a JWT in the Authorization header/)).toBeDefined()
  })

  it('renders generic bearer message when bearer_format is not JWT', () => {
    const scheme: SecurityScheme = {
      scheme_name: 'httpAuth',
      type: 'http',
      http_scheme: 'Bearer',
    }
    render(<SchemeCard scheme={scheme} />)
    expect(screen.getByText(/Expects a Bearer token in the Authorization header/)).toBeDefined()
  })

  it('renders API Key scheme', () => {
    const scheme: SecurityScheme = {
      scheme_name: 'apiKeyAuth',
      type: 'apiKey',
      api_key_location: 'header',
      api_key_name: 'X-API-Key',
      description: 'API Key in header',
    }
    const { container } = render(<SchemeCard scheme={scheme} />)
    expect(container).toMatchSnapshot()
  })

  it('renders OAuth2 scheme with flows', () => {
    const scheme: SecurityScheme = {
      scheme_name: 'oauth2Auth',
      type: 'oauth2',
      oauth_flows: [
        {
          flow_type: 'authorizationCode',
          authorization_url: 'https://auth.example.com/authorize',
          token_url: 'https://auth.example.com/token',
          scopes: { read: 'Read access', write: 'Write access' },
        },
      ],
    }
    const { container } = render(<SchemeCard scheme={scheme} />)
    expect(container).toMatchSnapshot()
  })

  it('does not render deprecated OAuth flows', () => {
    const scheme: SecurityScheme = {
      scheme_name: 'oauth2Auth',
      type: 'oauth2',
      oauth_flows: [
        { flow_type: 'implicit', authorization_url: 'https://auth.example.com/authorize', deprecated: true },
        { flow_type: 'clientCredentials', token_url: 'https://auth.example.com/token' },
      ],
    }
    const { container, queryByText } = render(<SchemeCard scheme={scheme} />)
    expect(queryByText(/implicit/i)).toBeNull()
    expect(container.textContent).toContain('Client Credentials')
  })

  it('renders openIdConnect scheme with discovery link', () => {
    const scheme: SecurityScheme = {
      scheme_name: 'oidcAuth',
      type: 'openIdConnect',
      openid_connect_url: 'https://auth.example.com/.well-known/openid-configuration',
    }
    render(<SchemeCard scheme={scheme} />)
    expect(screen.getByText('OpenID Connect Discovery')).toBeDefined()
    const link = document.querySelector('a[href*="openid-configuration"]')
    expect(link).toBeTruthy()
  })

  it('renders mutualTls scheme with mTLS message', () => {
    const scheme: SecurityScheme = {
      scheme_name: 'mtlsAuth',
      type: 'mutualTls',
    }
    render(<SchemeCard scheme={scheme} />)
    expect(screen.getByText(/requires mutual TLS/)).toBeDefined()
  })

  it('renders unknown scheme type with type as badge label', () => {
    const scheme: SecurityScheme = {
      scheme_name: 'customAuth',
      type: 'customScheme',
    }
    render(<SchemeCard scheme={scheme} />)
    // Badge should display the raw type name
    expect(screen.getByText('customScheme')).toBeDefined()
  })

  it('renders http scheme without bearer_format uses http_scheme in badge', () => {
    const scheme: SecurityScheme = {
      scheme_name: 'basicAuth',
      type: 'http',
      http_scheme: 'Basic',
    }
    render(<SchemeCard scheme={scheme} />)
    expect(screen.getByText('Basic')).toBeDefined()
  })

  it('renders oauth2 scheme with metadata URL', () => {
    const scheme: SecurityScheme = {
      scheme_name: 'oauth2Auth',
      type: 'oauth2',
      oauth2_metadata_url: 'https://auth.example.com/.well-known/oauth-authorization-server',
      oauth_flows: [],
    }
    render(<SchemeCard scheme={scheme} />)
    expect(screen.getByText('OAuth 2.0 Metadata')).toBeDefined()
  })
})

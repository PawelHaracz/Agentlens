import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
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
})

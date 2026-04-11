import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AuthenticationSection } from './AuthenticationSection'
import type { SecurityDetailView } from '@/lib/securityUtils'

describe('AuthenticationSection', () => {
  it('shows MCP transport message for mcp protocol', () => {
    render(<AuthenticationSection protocol="mcp" />)
    expect(screen.getByText(/MCP servers declare authentication at the transport level/)).toBeDefined()
  })

  it('shows no-auth message when securityDetail is undefined', () => {
    render(<AuthenticationSection protocol="a2a" />)
    expect(
      screen.getByText(/This agent does not declare any authentication requirements/),
    ).toBeDefined()
  })

  it('shows no-auth message when both schemes and requirements are empty', () => {
    const detail: SecurityDetailView = { security_schemes: [], security_requirements: [] }
    render(<AuthenticationSection protocol="a2a" securityDetail={detail} />)
    expect(
      screen.getByText(/This agent does not declare any authentication requirements/),
    ).toBeDefined()
  })

  it('does NOT show no-auth when requirements are present without schemes', () => {
    const detail: SecurityDetailView = {
      security_schemes: [],
      security_requirements: [{ schemes: { httpAuth: [] } }],
    }
    render(<AuthenticationSection protocol="a2a" securityDetail={detail} />)
    expect(screen.queryByText(/does not declare any authentication requirements/)).toBeNull()
  })

  it('renders scheme cards when security_schemes present', () => {
    const detail: SecurityDetailView = {
      security_schemes: [
        { scheme_name: 'httpAuth', type: 'http', http_scheme: 'Bearer', bearer_format: 'JWT' },
      ],
      security_requirements: [{ schemes: { httpAuth: [] } }],
    }
    render(
      <AuthenticationSection
        protocol="a2a"
        securityDetail={detail}
        endpoint="https://agent.example.com"
      />,
    )
    expect(screen.getByText('Authentication')).toBeDefined()
    // 'httpAuth' appears in both the banner and scheme card title — at least one match
    expect(screen.getAllByText('httpAuth').length).toBeGreaterThan(0)
  })

  it('renders requirements banner when requirements present', () => {
    const detail: SecurityDetailView = {
      security_schemes: [
        { scheme_name: 'httpAuth', type: 'http', http_scheme: 'Bearer', bearer_format: 'JWT' },
      ],
      security_requirements: [{ schemes: { httpAuth: [] } }],
    }
    render(
      <AuthenticationSection
        protocol="a2a"
        securityDetail={detail}
        endpoint="https://agent.example.com"
      />,
    )
    expect(screen.getByText('Required to connect')).toBeDefined()
  })

  it('renders connection recipe when endpoint and requirements present', () => {
    const detail: SecurityDetailView = {
      security_schemes: [
        { scheme_name: 'httpAuth', type: 'http', http_scheme: 'Bearer', bearer_format: 'JWT' },
      ],
      security_requirements: [{ schemes: { httpAuth: [] } }],
    }
    render(
      <AuthenticationSection
        protocol="a2a"
        securityDetail={detail}
        endpoint="https://agent.example.com"
      />,
    )
    expect(screen.getByText('Connection Example')).toBeDefined()
  })

  it('does NOT render connection recipe when no endpoint', () => {
    const detail: SecurityDetailView = {
      security_schemes: [
        { scheme_name: 'httpAuth', type: 'http', http_scheme: 'Bearer', bearer_format: 'JWT' },
      ],
      security_requirements: [{ schemes: { httpAuth: [] } }],
    }
    render(<AuthenticationSection protocol="a2a" securityDetail={detail} />)
    expect(screen.queryByText('Connection Example')).toBeNull()
  })

  it('does NOT render requirements banner when requirements empty', () => {
    const detail: SecurityDetailView = {
      security_schemes: [
        { scheme_name: 'httpAuth', type: 'http', http_scheme: 'Bearer', bearer_format: 'JWT' },
      ],
      security_requirements: [],
    }
    render(
      <AuthenticationSection
        protocol="a2a"
        securityDetail={detail}
        endpoint="https://agent.example.com"
      />,
    )
    expect(screen.queryByText('Required to connect')).toBeNull()
  })
})

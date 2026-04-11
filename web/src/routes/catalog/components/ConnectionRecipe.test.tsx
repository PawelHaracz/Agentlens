import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ConnectionRecipe } from './ConnectionRecipe'
import type { SecurityScheme, SecurityRequirement } from '@/lib/securityUtils'

// clipboard mock
const writeText = vi.fn()
Object.defineProperty(navigator, 'clipboard', {
  value: { writeText },
  writable: true,
})

const bearerScheme: SecurityScheme = {
  scheme_name: 'httpAuth',
  type: 'http',
  http_scheme: 'Bearer',
  bearer_format: 'JWT',
}

const apiKeyScheme: SecurityScheme = {
  scheme_name: 'apiKeyAuth',
  type: 'apiKey',
  api_key_location: 'header',
  api_key_name: 'X-API-Key',
}

const req: SecurityRequirement = { schemes: { httpAuth: [] } }

describe('ConnectionRecipe', () => {
  beforeEach(() => {
    writeText.mockClear()
  })

  it('renders the Connection Example heading', () => {
    render(
      <ConnectionRecipe
        endpoint="https://agent.example.com"
        requirements={[req]}
        schemes={[bearerScheme]}
      />,
    )
    expect(screen.getByText('Connection Example')).toBeDefined()
  })

  it('renders curl command with bearer header', () => {
    render(
      <ConnectionRecipe
        endpoint="https://agent.example.com"
        requirements={[req]}
        schemes={[bearerScheme]}
      />,
    )
    const code = screen.getByRole('code') ?? document.querySelector('code')
    expect(document.querySelector('code')?.textContent).toContain('curl')
    expect(document.querySelector('code')?.textContent).toContain('Authorization: Bearer')
    expect(document.querySelector('code')?.textContent).toContain('https://agent.example.com')
  })

  it('renders curl command with no requirements (bare curl)', () => {
    render(
      <ConnectionRecipe
        endpoint="https://agent.example.com"
        requirements={[]}
        schemes={[]}
      />,
    )
    expect(document.querySelector('code')?.textContent).toBe('curl https://agent.example.com')
  })

  it('copies curl to clipboard when copy button is clicked', () => {
    render(
      <ConnectionRecipe
        endpoint="https://agent.example.com"
        requirements={[req]}
        schemes={[bearerScheme]}
      />,
    )
    const btn = document.querySelector('button')!
    fireEvent.click(btn)
    expect(writeText).toHaveBeenCalledOnce()
    const calledWith = writeText.mock.calls[0][0] as string
    expect(calledWith).toContain('curl')
    expect(calledWith).toContain('Authorization: Bearer')
  })

  it('renders with apiKey scheme', () => {
    const apiKeyReq: SecurityRequirement = { schemes: { apiKeyAuth: [] } }
    render(
      <ConnectionRecipe
        endpoint="https://agent.example.com"
        requirements={[apiKeyReq]}
        schemes={[apiKeyScheme]}
      />,
    )
    expect(document.querySelector('code')?.textContent).toContain('X-API-Key')
  })
})

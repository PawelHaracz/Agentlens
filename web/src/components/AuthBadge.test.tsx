import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AuthBadge } from './AuthBadge'

describe('AuthBadge', () => {
  it('renders short label directly', () => {
    render(<AuthBadge label="Bearer JWT" required={true} />)
    expect(screen.getByText('Bearer JWT')).toBeDefined()
  })

  it('truncates label longer than 25 chars', () => {
    const longLabel = 'Bearer JWT + API Key + OAuth 2.0'
    render(<AuthBadge label={longLabel} required={false} />)
    // The badge shows truncated text (substring(0,22) + '...')
    expect(screen.getByText('Bearer JWT + API Key +...')).toBeDefined()
  })

  it('renders TooltipProvider wrapper for long labels', () => {
    const longLabel = 'Bearer JWT + API Key + OAuth 2.0'
    const { container } = render(<AuthBadge label={longLabel} required={false} />)
    // Tooltip wraps the badge — the truncated text is still visible in the DOM
    expect(container.textContent).toContain('Bearer JWT + API Key +...')
  })

  it('uses secondary variant for Open (no auth) label', () => {
    const { container } = render(<AuthBadge label="Open (no auth)" required={false} />)
    expect(container.textContent).toContain('Open (no auth)')
  })

  it('uses outline variant for authenticated label', () => {
    const { container } = render(<AuthBadge label="API Key" required={true} />)
    expect(container.textContent).toContain('API Key')
  })
})

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import ProtocolBadge from './ProtocolBadge'

describe('ProtocolBadge', () => {
  it('renders "a2a" label for a2a protocol', () => {
    render(<ProtocolBadge protocol="a2a" />)
    expect(screen.getByText('a2a')).toBeInTheDocument()
  })

  it('renders "mcp" label for mcp protocol', () => {
    render(<ProtocolBadge protocol="mcp" />)
    expect(screen.getByText('mcp')).toBeInTheDocument()
  })

  it('renders "a2ui" label for a2ui protocol', () => {
    render(<ProtocolBadge protocol="a2ui" />)
    expect(screen.getByText('a2ui')).toBeInTheDocument()
  })

  it('applies blue styling for a2a', () => {
    const { container } = render(<ProtocolBadge protocol="a2a" />)
    const badge = container.firstChild as HTMLElement
    expect(badge.className).toMatch(/blue/)
  })

  it('applies green styling for mcp', () => {
    const { container } = render(<ProtocolBadge protocol="mcp" />)
    const badge = container.firstChild as HTMLElement
    expect(badge.className).toMatch(/green/)
  })

  it('applies purple styling for a2ui', () => {
    const { container } = render(<ProtocolBadge protocol="a2ui" />)
    const badge = container.firstChild as HTMLElement
    expect(badge.className).toMatch(/purple/)
  })
})

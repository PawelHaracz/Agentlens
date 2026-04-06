import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import StatusBadge from './StatusBadge'

describe('StatusBadge', () => {
  it('renders "healthy" text', () => {
    render(<StatusBadge status="healthy" />)
    expect(screen.getByText('healthy')).toBeInTheDocument()
  })

  it('renders "degraded" text', () => {
    render(<StatusBadge status="degraded" />)
    expect(screen.getByText('degraded')).toBeInTheDocument()
  })

  it('renders "down" text', () => {
    render(<StatusBadge status="down" />)
    expect(screen.getByText('down')).toBeInTheDocument()
  })

  it('renders "unknown" text', () => {
    render(<StatusBadge status="unknown" />)
    expect(screen.getByText('unknown')).toBeInTheDocument()
  })

  it('applies green styling for healthy status', () => {
    const { container } = render(<StatusBadge status="healthy" />)
    const badge = container.firstChild as HTMLElement
    expect(badge.className).toMatch(/green/)
  })

  it('applies yellow styling for degraded status', () => {
    const { container } = render(<StatusBadge status="degraded" />)
    const badge = container.firstChild as HTMLElement
    expect(badge.className).toMatch(/yellow/)
  })
})

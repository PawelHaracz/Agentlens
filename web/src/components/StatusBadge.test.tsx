import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import StatusBadge from './StatusBadge'

describe('StatusBadge', () => {
  it('renders "Active" label for active status', () => {
    render(<StatusBadge status="active" />)
    expect(screen.getByText('Active')).toBeInTheDocument()
  })

  it('renders "Degraded" label for degraded status', () => {
    render(<StatusBadge status="degraded" />)
    expect(screen.getByText('Degraded')).toBeInTheDocument()
  })

  it('renders "Offline" label for offline status', () => {
    render(<StatusBadge status="offline" />)
    expect(screen.getByText('Offline')).toBeInTheDocument()
  })

  it('renders "Pending" label for registered status', () => {
    render(<StatusBadge status="registered" />)
    expect(screen.getByText('Pending')).toBeInTheDocument()
  })

  it('renders "Deprecated" label for deprecated status', () => {
    render(<StatusBadge status="deprecated" />)
    expect(screen.getByText('Deprecated')).toBeInTheDocument()
  })

  it('applies green styling for active status', () => {
    render(<StatusBadge status="active" />)
    const badge = screen.getByText('Active')
    expect(badge.className).toMatch(/green/)
  })

  it('applies yellow styling for degraded status', () => {
    render(<StatusBadge status="degraded" />)
    const badge = screen.getByText('Degraded')
    expect(badge.className).toMatch(/yellow/)
  })

  it('shows latency when active and latencyMs provided', () => {
    render(<StatusBadge status="active" latencyMs={42} />)
    expect(screen.getByText('42 ms')).toBeInTheDocument()
  })

  it('does not show latency when latencyMs is 0', () => {
    render(<StatusBadge status="active" latencyMs={0} />)
    expect(screen.queryByText(/ms/)).not.toBeInTheDocument()
  })
})

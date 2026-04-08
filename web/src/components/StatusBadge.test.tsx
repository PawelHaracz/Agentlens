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

  it('applies slate styling for deprecated status', () => {
    render(<StatusBadge status="deprecated" />)
    const badge = screen.getByText('Deprecated')
    expect(badge.className).toMatch(/slate/)
  })

  it('shows latency when active and latencyMs provided', () => {
    render(<StatusBadge status="active" latencyMs={42} />)
    expect(screen.getByText('42 ms')).toBeInTheDocument()
  })

  it('shows latency when degraded and latencyMs provided', () => {
    render(<StatusBadge status="degraded" latencyMs={500} />)
    expect(screen.getByText('500 ms')).toBeInTheDocument()
  })

  it('does not show latency when latencyMs is 0', () => {
    render(<StatusBadge status="active" latencyMs={0} />)
    expect(screen.queryByText(/ms/)).not.toBeInTheDocument()
  })

  it('does not show latency for offline status even with latencyMs', () => {
    render(<StatusBadge status="offline" latencyMs={100} />)
    expect(screen.queryByText(/ms/)).not.toBeInTheDocument()
  })

  it('shows relative time when lastSeenAt is provided', () => {
    const recent = new Date(Date.now() - 30_000).toISOString() // 30s ago
    render(<StatusBadge status="active" lastSeenAt={recent} />)
    expect(screen.getByText(/ago/)).toBeInTheDocument()
  })

  it('shows hours-relative time for older lastSeenAt', () => {
    const old = new Date(Date.now() - 7_200_000).toISOString() // 2h ago
    render(<StatusBadge status="active" lastSeenAt={old} />)
    expect(screen.getByText(/h ago/)).toBeInTheDocument()
  })

  it('shows minutes-relative time for mid-age lastSeenAt', () => {
    const mid = new Date(Date.now() - 150_000).toISOString() // 2.5m ago
    render(<StatusBadge status="active" lastSeenAt={mid} />)
    expect(screen.getByText(/m ago/)).toBeInTheDocument()
  })

  it('does not show relative time when lastSeenAt is not provided', () => {
    render(<StatusBadge status="active" />)
    expect(screen.queryByText(/ago/)).not.toBeInTheDocument()
  })

  it('falls back to registered config for unknown status', () => {
    // @ts-expect-error testing unknown value
    render(<StatusBadge status="unknown-status" />)
    expect(screen.getByText('Pending')).toBeInTheDocument()
  })
})

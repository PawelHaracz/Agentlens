import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import StatsBar from './StatsBar'
import type { Stats } from '../types'

const makeStats = (overrides: Partial<Stats> = {}): Stats => ({
  total: 0,
  by_status: {},
  by_source: {},
  ...overrides,
})

describe('StatsBar', () => {
  it('renders total stat', () => {
    render(<StatsBar stats={makeStats({ total: 42 })} />)
    expect(screen.getByText('42')).toBeInTheDocument()
    expect(screen.getByText('Total')).toBeInTheDocument()
  })

  it('renders healthy count', () => {
    render(<StatsBar stats={makeStats({ by_status: { healthy: 10 } })} />)
    expect(screen.getByText('10')).toBeInTheDocument()
    expect(screen.getByText('Healthy')).toBeInTheDocument()
  })

  it('renders degraded count', () => {
    render(<StatsBar stats={makeStats({ by_status: { degraded: 3 } })} />)
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('Degraded')).toBeInTheDocument()
  })

  it('renders Down stat combining down and unknown', () => {
    render(<StatsBar stats={makeStats({ by_status: { down: 2, unknown: 1 } })} />)
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('Down')).toBeInTheDocument()
  })

  it('renders zeros when by_status is empty', () => {
    render(<StatsBar stats={makeStats({ total: 0 })} />)
    const zeros = screen.getAllByText('0')
    expect(zeros.length).toBeGreaterThanOrEqual(4)
  })
})

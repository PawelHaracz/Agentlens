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

  it('renders active count', () => {
    render(<StatsBar stats={makeStats({ by_status: { active: 10 } })} />)
    expect(screen.getByText('10')).toBeInTheDocument()
    expect(screen.getByText('Active')).toBeInTheDocument()
  })

  it('renders degraded count', () => {
    render(<StatsBar stats={makeStats({ by_status: { degraded: 3 } })} />)
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('Degraded')).toBeInTheDocument()
  })

  it('renders Offline stat combining offline and registered', () => {
    render(<StatsBar stats={makeStats({ by_status: { offline: 2, registered: 1 } })} />)
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('Offline')).toBeInTheDocument()
  })

  it('renders zeros when by_status is empty', () => {
    render(<StatsBar stats={makeStats({ total: 0 })} />)
    const zeros = screen.getAllByText('0')
    expect(zeros.length).toBeGreaterThanOrEqual(4)
  })
})

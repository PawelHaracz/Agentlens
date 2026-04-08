import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, vi, expect } from 'vitest'
import { ProtocolFilter } from './ProtocolFilter'

describe('ProtocolFilter', () => {
  it('renders All, A2A, MCP toggle buttons', () => {
    render(<ProtocolFilter value={undefined} onChange={vi.fn()} />)
    expect(screen.getByRole('radio', { name: 'All' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'A2A' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'MCP' })).toBeInTheDocument()
  })

  it('calls onChange with undefined when All is clicked', () => {
    const onChange = vi.fn()
    render(<ProtocolFilter value="a2a" onChange={onChange} />)
    fireEvent.click(screen.getByRole('radio', { name: 'All' }))
    expect(onChange).toHaveBeenCalledWith(undefined)
  })

  it('calls onChange with "a2a" when A2A is clicked', () => {
    const onChange = vi.fn()
    render(<ProtocolFilter value={undefined} onChange={onChange} />)
    fireEvent.click(screen.getByRole('radio', { name: 'A2A' }))
    expect(onChange).toHaveBeenCalledWith('a2a')
  })

  it('calls onChange with "mcp" when MCP is clicked', () => {
    const onChange = vi.fn()
    render(<ProtocolFilter value={undefined} onChange={onChange} />)
    fireEvent.click(screen.getByRole('radio', { name: 'MCP' }))
    expect(onChange).toHaveBeenCalledWith('mcp')
  })

  it('reflects current value by selecting the correct radio', () => {
    render(<ProtocolFilter value="mcp" onChange={vi.fn()} />)
    const mcp = screen.getByRole('radio', { name: 'MCP' })
    expect(mcp).toHaveAttribute('data-state', 'on')
  })

  it('selects All when value is undefined', () => {
    render(<ProtocolFilter value={undefined} onChange={vi.fn()} />)
    const all = screen.getByRole('radio', { name: 'All' })
    expect(all).toHaveAttribute('data-state', 'on')
  })
})

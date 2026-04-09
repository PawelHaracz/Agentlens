import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { KindFilter } from './KindFilter'

describe('KindFilter', () => {
  it('renders all kind options', () => {
    render(<KindFilter value="" onChange={() => {}} />)
    expect(screen.getByText('All')).toBeInTheDocument()
    expect(screen.getByText('A2A Skill')).toBeInTheDocument()
    expect(screen.getByText('MCP Tool')).toBeInTheDocument()
    expect(screen.getByText('MCP Resource')).toBeInTheDocument()
    expect(screen.getByText('MCP Prompt')).toBeInTheDocument()
  })

  it('calls onChange with empty string when All is selected', () => {
    const onChange = vi.fn()
    render(<KindFilter value="a2a.skill" onChange={onChange} />)
    fireEvent.click(screen.getByText('All'))
    expect(onChange).toHaveBeenCalledWith('')
  })

  it('calls onChange with kind value when a kind is clicked', () => {
    const onChange = vi.fn()
    render(<KindFilter value="" onChange={onChange} />)
    fireEvent.click(screen.getByText('MCP Tool'))
    expect(onChange).toHaveBeenCalledWith('mcp.tool')
  })

  it('has correct aria-label', () => {
    const { container } = render(<KindFilter value="" onChange={() => {}} />)
    const group = container.querySelector('[aria-label="Filter by capability kind"]')
    expect(group).toBeTruthy()
  })
})

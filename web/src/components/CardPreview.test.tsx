import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import CardPreview from './CardPreview'
import type { ValidationPreview } from '../types'

const makePreview = (overrides: Partial<ValidationPreview> = {}): ValidationPreview => ({
  display_name: 'My Agent',
  description: 'A useful agent',
  protocol: 'a2a',
  ...overrides,
})

describe('CardPreview', () => {
  it('renders the display_name', () => {
    render(<CardPreview preview={makePreview({ display_name: 'Awesome Bot' })} />)
    expect(screen.getByText('Awesome Bot')).toBeInTheDocument()
  })

  it('renders the description when provided', () => {
    render(<CardPreview preview={makePreview({ description: 'Does cool things' })} />)
    expect(screen.getByText('Does cool things')).toBeInTheDocument()
  })

  it('renders the protocol badge in uppercase', () => {
    render(<CardPreview preview={makePreview({ protocol: 'a2a' })} />)
    expect(screen.getByText('A2A')).toBeInTheDocument()
  })

  it('renders spec_version badge when provided', () => {
    render(<CardPreview preview={makePreview({ spec_version: '1.2.3' })} />)
    expect(screen.getByText('v1.2.3')).toBeInTheDocument()
  })

  it('does not render spec_version badge when absent', () => {
    render(<CardPreview preview={makePreview({ spec_version: undefined })} />)
    expect(screen.queryByText(/^v/)).not.toBeInTheDocument()
  })

  it('does not render description when absent', () => {
    render(<CardPreview preview={makePreview({ description: '' })} />)
    expect(screen.queryByText('A useful agent')).not.toBeInTheDocument()
  })

  it('renders mcp protocol badge', () => {
    render(<CardPreview preview={makePreview({ protocol: 'mcp' })} />)
    expect(screen.getByText('MCP')).toBeInTheDocument()
  })
})

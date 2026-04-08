import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { SpecVersionBadge } from './SpecVersionBadge'

describe('SpecVersionBadge', () => {
  it('renders spec version text', () => {
    render(<SpecVersionBadge version="0.7" />)
    expect(screen.getByText('0.7')).toBeInTheDocument()
  })
  it('renders nothing for empty version', () => {
    const { container } = render(<SpecVersionBadge />)
    expect(container.firstChild).toBeNull()
  })
  it('renders nothing for undefined version', () => {
    const { container } = render(<SpecVersionBadge version={undefined} />)
    expect(container.firstChild).toBeNull()
  })
})

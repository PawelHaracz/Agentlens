import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { SearchHighlight } from './SearchHighlight'

describe('SearchHighlight', () => {
  it('renders snippet', () => {
    render(<SearchHighlight snippet="translate between en and de" />)
    expect(screen.getByText(/translate between en and de/)).toBeInTheDocument()
  })
  it('renders nothing when no snippet', () => {
    const { container } = render(<SearchHighlight />)
    expect(container.firstChild).toBeNull()
  })
})

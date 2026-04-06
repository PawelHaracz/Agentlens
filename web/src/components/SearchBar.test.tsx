import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import SearchBar from './SearchBar'

describe('SearchBar', () => {
  it('renders the search input with default placeholder', () => {
    render(<SearchBar value="" onChange={vi.fn()} />)
    expect(screen.getByPlaceholderText('Search catalog…')).toBeInTheDocument()
  })

  it('renders a custom placeholder when provided', () => {
    render(<SearchBar value="" onChange={vi.fn()} placeholder="Find agents…" />)
    expect(screen.getByPlaceholderText('Find agents…')).toBeInTheDocument()
  })

  it('displays the current value', () => {
    render(<SearchBar value="hello" onChange={vi.fn()} />)
    const input = screen.getByRole('textbox') as HTMLInputElement
    expect(input.value).toBe('hello')
  })

  it('calls onChange when the user types', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<SearchBar value="" onChange={onChange} />)
    await user.type(screen.getByRole('textbox'), 'abc')
    expect(onChange).toHaveBeenCalledTimes(3)
    expect(onChange).toHaveBeenNthCalledWith(1, 'a')
    expect(onChange).toHaveBeenNthCalledWith(2, 'b')
    expect(onChange).toHaveBeenNthCalledWith(3, 'c')
    expect(onChange).toHaveBeenLastCalledWith('c')
  })

  it('calls onChange with empty string when input is cleared', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<SearchBar value="foo" onChange={onChange} />)
    await user.clear(screen.getByRole('textbox'))
    expect(onChange).toHaveBeenLastCalledWith('')
  })
})

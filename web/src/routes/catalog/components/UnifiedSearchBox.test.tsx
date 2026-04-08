import { render, screen, fireEvent, act } from '@testing-library/react'
import { describe, it, vi, beforeEach, afterEach, expect } from 'vitest'
import { UnifiedSearchBox } from './UnifiedSearchBox'

describe('UnifiedSearchBox', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders search input with placeholder', () => {
    render(<UnifiedSearchBox value="" onChange={vi.fn()} />)
    expect(screen.getByPlaceholderText(/search across/i)).toBeInTheDocument()
  })

  it('renders with initial value', () => {
    render(<UnifiedSearchBox value="hello" onChange={vi.fn()} />)
    expect(screen.getByRole('textbox', { name: /search catalog/i })).toHaveValue('hello')
  })

  it('calls onChange after debounce timeout', async () => {
    const onChange = vi.fn()
    render(<UnifiedSearchBox value="" onChange={onChange} />)
    const input = screen.getByRole('textbox', { name: /search catalog/i })
    fireEvent.change(input, { target: { value: 'test' } })
    expect(onChange).not.toHaveBeenCalled()
    act(() => { vi.advanceTimersByTime(250) })
    expect(onChange).toHaveBeenCalledWith('test')
  })

  it('calls onChange immediately on Enter key', () => {
    const onChange = vi.fn()
    render(<UnifiedSearchBox value="" onChange={onChange} />)
    const input = screen.getByRole('textbox', { name: /search catalog/i })
    fireEvent.change(input, { target: { value: 'query' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onChange).toHaveBeenCalledWith('query')
  })

  it('blurs input on Escape key', () => {
    render(<UnifiedSearchBox value="" onChange={vi.fn()} />)
    const input = screen.getByRole('textbox', { name: /search catalog/i })
    const blurSpy = vi.spyOn(input, 'blur')
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(blurSpy).toHaveBeenCalled()
  })

  it('shows clear button when draft is non-empty', () => {
    render(<UnifiedSearchBox value="hello" onChange={vi.fn()} />)
    expect(screen.getByRole('button', { name: /clear search/i })).toBeInTheDocument()
  })

  it('does not show clear button when draft is empty', () => {
    render(<UnifiedSearchBox value="" onChange={vi.fn()} />)
    expect(screen.queryByRole('button', { name: /clear search/i })).not.toBeInTheDocument()
  })

  it('calls onChange with empty string on clear', () => {
    const onChange = vi.fn()
    render(<UnifiedSearchBox value="hello" onChange={onChange} />)
    const clearBtn = screen.getByRole('button', { name: /clear search/i })
    fireEvent.click(clearBtn)
    expect(onChange).toHaveBeenCalledWith('')
  })

  it('updates draft when value prop changes externally', () => {
    const { rerender } = render(<UnifiedSearchBox value="first" onChange={vi.fn()} />)
    rerender(<UnifiedSearchBox value="second" onChange={vi.fn()} />)
    expect(screen.getByRole('textbox', { name: /search catalog/i })).toHaveValue('second')
  })

  it('pressing "/" key focuses the input when not already focused', () => {
    render(<UnifiedSearchBox value="" onChange={vi.fn()} />)
    const input = screen.getByRole('textbox', { name: /search catalog/i })
    const focusSpy = vi.spyOn(input, 'focus')
    fireEvent.keyDown(document, { key: '/' })
    expect(focusSpy).toHaveBeenCalled()
  })
})

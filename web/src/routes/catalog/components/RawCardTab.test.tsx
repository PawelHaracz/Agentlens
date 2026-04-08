import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, vi, beforeEach, expect } from 'vitest'
import { RawCardTab } from './RawCardTab'
import * as api from '../../../api'

vi.mock('../../../api', () => ({
  getRawCard: vi.fn(),
}))

vi.mock('prismjs', () => ({ default: { highlightAll: vi.fn() } }))
vi.mock('prismjs/components/prism-json', () => ({}))

const FETCHED_AT = '2025-01-01T12:00:00.000Z'
const CONTENT_TYPE = 'application/json'

function mockCard(overrides: Partial<{ data: string; truncated: boolean; fetchedAt: string }> = {}) {
  return {
    data: '{"name":"agent","version":"1.0"}',
    contentType: CONTENT_TYPE,
    fetchedAt: FETCHED_AT,
    truncated: false,
    ...overrides,
  }
}

describe('RawCardTab', () => {
  beforeEach(() => {
    vi.spyOn(api, 'getRawCard').mockResolvedValue(mockCard())
  })

  it('shows loading skeleton initially', () => {
    vi.spyOn(api, 'getRawCard').mockReturnValue(new Promise(() => {}))
    render(<RawCardTab entryId="e1" displayName="My Agent" />)
    expect(document.querySelector('.animate-pulse')).toBeTruthy()
  })

  it('renders JSON content after load', async () => {
    render(<RawCardTab entryId="e1" displayName="My Agent" />)
    await screen.findByText(/agent/)
  })

  it('renders Copy and Download buttons', async () => {
    render(<RawCardTab entryId="e1" displayName="My Agent" />)
    await screen.findByRole('button', { name: /copy/i })
    expect(screen.getByRole('button', { name: /download/i })).toBeInTheDocument()
  })

  it('renders fetchedAt timestamp', async () => {
    render(<RawCardTab entryId="e1" displayName="My Agent" />)
    await screen.findByText(/fetched at/i)
  })

  it('does NOT show truncation alert when truncated is false', async () => {
    render(<RawCardTab entryId="e1" displayName="My Agent" />)
    await screen.findByRole('button', { name: /copy/i })
    expect(screen.queryByText(/truncated/i)).not.toBeInTheDocument()
  })

  it('shows truncation alert when truncated is true', async () => {
    vi.spyOn(api, 'getRawCard').mockResolvedValue(mockCard({ data: '{"x":1}', truncated: true }))
    render(<RawCardTab entryId="e1" displayName="My Agent" />)
    await screen.findByText(/truncated/i)
  })

  it('shows error state when fetch fails', async () => {
    vi.spyOn(api, 'getRawCard').mockRejectedValue(new Error('Not found'))
    render(<RawCardTab entryId="e1" displayName="My Agent" />)
    await screen.findByText('Not found')
  })

  it('handles invalid JSON gracefully (shows raw data)', async () => {
    vi.spyOn(api, 'getRawCard').mockResolvedValue(mockCard({ data: 'not json {{{' }))
    render(<RawCardTab entryId="e1" displayName="My Agent" />)
    await screen.findByText('not json {{{')
  })

  it('re-fetches when entryId changes', async () => {
    const { rerender } = render(<RawCardTab entryId="e1" displayName="Agent 1" />)
    await screen.findByRole('button', { name: /copy/i })
    vi.spyOn(api, 'getRawCard').mockResolvedValue(mockCard({ data: '{"changed":true}' }))
    rerender(<RawCardTab entryId="e2" displayName="Agent 2" />)
    await waitFor(() => expect(api.getRawCard).toHaveBeenCalledWith('e2'))
  })

  it('Download button creates anchor click', async () => {
    const clickSpy = vi.fn()
    const createElementOrig = document.createElement.bind(document)
    vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = createElementOrig(tag)
      if (tag === 'a') {
        vi.spyOn(el as HTMLAnchorElement, 'click').mockImplementation(clickSpy)
      }
      return el
    })
    const createURL = vi.fn().mockReturnValue('blob:fake')
    const revokeURL = vi.fn()
    vi.stubGlobal('URL', { createObjectURL: createURL, revokeObjectURL: revokeURL })

    render(<RawCardTab entryId="e1" displayName="My Agent" />)
    const btn = await screen.findByRole('button', { name: /download/i })
    fireEvent.click(btn)
    expect(clickSpy).toHaveBeenCalled()

    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('Copy button calls navigator.clipboard.writeText', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
      writable: true,
    })
    render(<RawCardTab entryId="e1" displayName="My Agent" />)
    const btn = await screen.findByRole('button', { name: /copy/i })
    fireEvent.click(btn)
    await waitFor(() => expect(writeText).toHaveBeenCalled())
    await screen.findByText('Copied!')
  })
})

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import EntryDetail from './EntryDetail'
import type { CatalogEntry } from '../types'

vi.mock('../api', () => ({
  getEntry: vi.fn(),
  deleteEntry: vi.fn(),
}))

import { getEntry, deleteEntry } from '../api'

const mockGetEntry = getEntry as ReturnType<typeof vi.fn>
const mockDeleteEntry = deleteEntry as ReturnType<typeof vi.fn>

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

const baseEntry: CatalogEntry = {
  id: 'entry-1',
  display_name: 'My Service',
  description: 'A great service',
  protocol: 'a2a',
  endpoint: 'https://example.com/service',
  version: '2.0.0',
  status: 'healthy',
  source: 'push',
  agent_type_id: 'type-1',
  validity: { last_seen: '2026-01-01T00:00:00Z' },
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
  window.confirm = vi.fn().mockReturnValue(true)
})

function renderEntryDetail(id = 'entry-1') {
  return render(
    <MemoryRouter initialEntries={[`/catalog/${id}`]}>
      <Routes>
        <Route path="/catalog/:id" element={<EntryDetail />} />
        <Route path="/" element={<div>Home</div>} />
      </Routes>
    </MemoryRouter>
  )
}

describe('EntryDetail', () => {
  it('shows loading skeleton while fetching', () => {
    mockGetEntry.mockReturnValue(new Promise(() => {}))
    const { container } = renderEntryDetail()
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument()
  })

  it('renders entry display_name after fetch', async () => {
    mockGetEntry.mockResolvedValue(baseEntry)
    renderEntryDetail()
    await waitFor(() => {
      expect(screen.getByText('My Service')).toBeInTheDocument()
    })
  })

  it('renders entry description', async () => {
    mockGetEntry.mockResolvedValue(baseEntry)
    renderEntryDetail()
    await waitFor(() => {
      expect(screen.getByText('A great service')).toBeInTheDocument()
    })
  })

  it('renders the endpoint', async () => {
    mockGetEntry.mockResolvedValue(baseEntry)
    renderEntryDetail()
    await waitFor(() => {
      expect(screen.getByText('https://example.com/service')).toBeInTheDocument()
    })
  })

  it('renders protocol and status badges', async () => {
    mockGetEntry.mockResolvedValue(baseEntry)
    renderEntryDetail()
    await waitFor(() => {
      expect(screen.getByText('a2a')).toBeInTheDocument()
      expect(screen.getByText('healthy')).toBeInTheDocument()
    })
  })

  it('shows error card on fetch failure', async () => {
    mockGetEntry.mockRejectedValue(new Error('Not found'))
    renderEntryDetail()
    await waitFor(() => {
      expect(screen.getByText('Not found')).toBeInTheDocument()
    })
  })

  it('renders capabilities section when present', async () => {
    mockGetEntry.mockResolvedValue({
      ...baseEntry,
      capabilities: [
        { kind: 'a2a.skill', name: 'MySkill', description: 'Does things' },
      ],
    })
    renderEntryDetail()
    await waitFor(() => {
      expect(screen.getByText('MySkill')).toBeInTheDocument()
      expect(screen.getByText('a2a.skill')).toBeInTheDocument()
    })
  })

  it('renders raw definition section when present', async () => {
    mockGetEntry.mockResolvedValue({
      ...baseEntry,
      raw_definition: { name: 'test', version: '1.0' },
    })
    renderEntryDetail()
    await waitFor(() => {
      expect(screen.getByText(/raw definition/i)).toBeInTheDocument()
    })
  })

  it('renders categories when present', async () => {
    mockGetEntry.mockResolvedValue({
      ...baseEntry,
      categories: ['nlp', 'search'],
    })
    renderEntryDetail()
    await waitFor(() => {
      expect(screen.getByText('nlp')).toBeInTheDocument()
      expect(screen.getByText('search')).toBeInTheDocument()
    })
  })

  it('calls deleteEntry and navigates home on delete confirmation', async () => {
    const user = userEvent.setup()
    mockGetEntry.mockResolvedValue(baseEntry)
    mockDeleteEntry.mockResolvedValue(undefined)
    renderEntryDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /delete/i }))

    await waitFor(() => {
      expect(mockDeleteEntry).toHaveBeenCalledWith('entry-1')
      expect(mockNavigate).toHaveBeenCalledWith('/')
    })
  })

  it('shows error when delete fails', async () => {
    const user = userEvent.setup()
    mockGetEntry.mockResolvedValue(baseEntry)
    mockDeleteEntry.mockRejectedValue(new Error('Delete failed'))
    renderEntryDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /delete/i }))

    await waitFor(() => {
      expect(screen.getByText('Delete failed')).toBeInTheDocument()
    })
  })

  it('renders the back to catalog link', async () => {
    mockGetEntry.mockResolvedValue(baseEntry)
    renderEntryDetail()
    await waitFor(() => {
      expect(screen.getByRole('link', { name: /back to catalog/i })).toBeInTheDocument()
    })
  })
})

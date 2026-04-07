import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import CatalogList from './CatalogList'
import type { CatalogEntry, Stats } from '../types'

vi.mock('../api', () => ({
  listCatalog: vi.fn(),
  getStats: vi.fn(),
}))

vi.mock('./RegisterAgentDialog', () => ({
  default: ({ onRegistered }: { onRegistered: () => void }) => (
    <button onClick={onRegistered}>Register Agent</button>
  ),
}))

import { listCatalog, getStats } from '../api'

const mockListCatalog = listCatalog as ReturnType<typeof vi.fn>
const mockGetStats = getStats as ReturnType<typeof vi.fn>

const mockStats: Stats = {
  total: 2,
  by_status: { healthy: 1, degraded: 1 },
  by_source: { push: 2 },
}

const makeEntry = (overrides: Partial<CatalogEntry> = {}): CatalogEntry => ({
  id: 'abc-1',
  display_name: 'Test Agent',
  description: 'A test agent',
  protocol: 'a2a',
  endpoint: 'https://example.com/agent',
  version: '1.0.0',
  status: 'active',
  source: 'push',
  agent_type_id: 'type-1',
  validity: { last_seen: '2026-01-01T00:00:00Z' },
  health: {
    state: 'active',
    latencyMs: 0,
    consecutiveFailures: 0,
    lastError: '',
    lastProbedAt: '2026-01-01T00:00:00Z',
    lastSuccessAt: '2026-01-01T00:00:00Z',
  },
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  ...overrides,
})

beforeEach(() => {
  vi.clearAllMocks()
  mockGetStats.mockResolvedValue(mockStats)
})

function renderCatalogList() {
  return render(
    <MemoryRouter>
      <CatalogList />
    </MemoryRouter>
  )
}

describe('CatalogList', () => {
  it('shows loading skeletons while fetching', () => {
    mockListCatalog.mockReturnValue(new Promise(() => {}))
    const { container } = renderCatalogList()
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument()
  })

  it('renders entries after successful fetch', async () => {
    mockListCatalog.mockResolvedValue([makeEntry()])
    renderCatalogList()
    await waitFor(() => {
      expect(screen.getByText('Test Agent')).toBeInTheDocument()
    })
  })

  it('renders a link to the entry detail page', async () => {
    mockListCatalog.mockResolvedValue([makeEntry({ id: 'abc-1', display_name: 'My Bot' })])
    renderCatalogList()
    await waitFor(() => {
      const link = screen.getByRole('link', { name: 'My Bot' })
      expect(link).toHaveAttribute('href', '/catalog/abc-1')
    })
  })

  it('shows empty state when no entries', async () => {
    mockListCatalog.mockResolvedValue([])
    renderCatalogList()
    await waitFor(() => {
      expect(screen.getByText(/no catalog entries found/i)).toBeInTheDocument()
    })
  })

  it('shows error message on fetch failure', async () => {
    mockListCatalog.mockRejectedValue(new Error('Network error'))
    renderCatalogList()
    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument()
    })
  })

  it('renders stats bar when stats are available', async () => {
    mockListCatalog.mockResolvedValue([])
    renderCatalogList()
    await waitFor(() => {
      expect(screen.getByText('Total')).toBeInTheDocument()
    })
  })

  it('renders multiple entries', async () => {
    mockListCatalog.mockResolvedValue([
      makeEntry({ id: '1', display_name: 'Agent Alpha' }),
      makeEntry({ id: '2', display_name: 'Agent Beta' }),
    ])
    renderCatalogList()
    await waitFor(() => {
      expect(screen.getByText('Agent Alpha')).toBeInTheDocument()
      expect(screen.getByText('Agent Beta')).toBeInTheDocument()
    })
  })

  it('renders protocol and status badges for each entry', async () => {
    mockListCatalog.mockResolvedValue([makeEntry({ protocol: 'mcp', status: 'degraded' })])
    renderCatalogList()
    await waitFor(() => {
      expect(screen.getByText('mcp')).toBeInTheDocument()
      expect(screen.getAllByText('Degraded').length).toBeGreaterThan(0)
    })
  })

  it('renders search bar', async () => {
    mockListCatalog.mockResolvedValue([])
    renderCatalogList()
    await waitFor(() => {
      expect(screen.getByPlaceholderText(/search catalog/i)).toBeInTheDocument()
    })
  })

  it('reloads entries when search text changes', async () => {
    const user = userEvent.setup()
    mockListCatalog.mockResolvedValue([])
    renderCatalogList()
    await waitFor(() => {
      expect(mockListCatalog).toHaveBeenCalledTimes(1)
    })
    await user.type(screen.getByPlaceholderText(/search catalog/i), 'bot')
    await waitFor(() => {
      expect(mockListCatalog).toHaveBeenCalledWith(expect.objectContaining({ q: 'b' }))
    })
  })

  it('reloads entries when register callback is triggered', async () => {
    const user = userEvent.setup()
    mockListCatalog.mockResolvedValue([])
    renderCatalogList()

    await waitFor(() => {
      expect(mockListCatalog).toHaveBeenCalledTimes(1)
    })

    await user.click(screen.getByRole('button', { name: /register agent/i }))

    await waitFor(() => {
      expect(mockListCatalog).toHaveBeenCalledTimes(2)
    })
  })

  it('filters by lifecycle state and can clear filters', async () => {
    const user = userEvent.setup()
    mockListCatalog.mockResolvedValue([])
    renderCatalogList()

    await waitFor(() => {
      expect(mockListCatalog).toHaveBeenCalledTimes(1)
    })

    await user.click(screen.getByRole('button', { name: /all statuses/i }))
    await user.click(screen.getByRole('menuitemcheckbox', { name: 'Active' }))

    await waitFor(() => {
      expect(mockListCatalog).toHaveBeenCalledWith(expect.objectContaining({ state: 'active' }))
    })

    await waitFor(() => {
      expect(screen.getByText(/no entries match the selected status filter/i)).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /clear filters/i }))

    await waitFor(() => {
      expect(mockListCatalog).toHaveBeenLastCalledWith(expect.objectContaining({ state: undefined }))
    })
  })
})

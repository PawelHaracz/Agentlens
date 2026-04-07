import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import EntryDetail from './EntryDetail'
import { AuthProvider } from '../contexts/AuthContext'
import type { CatalogEntry } from '../types'

vi.mock('../api', () => ({
  getEntry: vi.fn(),
  deleteEntry: vi.fn(),
  patchLifecycle: vi.fn(),
  postProbe: vi.fn(),
  getMe: vi.fn().mockResolvedValue({
    id: 'user-1',
    username: 'admin',
    email: 'admin@example.com',
    display_name: 'Admin',
    role_id: 'role-1',
    role: {
      id: 'role-1',
      name: 'admin',
      description: 'Admin',
      permissions: ['catalog:write', 'catalog:read'],
      is_system: true,
    },
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  }),
  setToken: vi.fn(),
  getToken: vi.fn(),
}))

import { getEntry, deleteEntry, postProbe, patchLifecycle } from '../api'

const mockGetEntry = getEntry as ReturnType<typeof vi.fn>
const mockDeleteEntry = deleteEntry as ReturnType<typeof vi.fn>
const mockPostProbe = postProbe as ReturnType<typeof vi.fn>
const mockPatchLifecycle = patchLifecycle as ReturnType<typeof vi.fn>

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
}

let confirmSpy: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  vi.clearAllMocks()
  confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
})

afterEach(() => {
  confirmSpy.mockRestore()
})

function renderEntryDetail(id = 'entry-1') {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={[`/catalog/${id}`]}>
        <Routes>
          <Route path="/catalog/:id" element={<EntryDetail />} />
          <Route path="/" element={<div>Home</div>} />
        </Routes>
      </MemoryRouter>
    </AuthProvider>
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
      expect(screen.getAllByText('Active').length).toBeGreaterThan(0)
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

  it('renders health section with state', async () => {
    mockGetEntry.mockResolvedValue(baseEntry)
    renderEntryDetail()
    await waitFor(() => {
      expect(screen.getByText('Health')).toBeInTheDocument()
      expect(screen.getByText('State')).toBeInTheDocument()
    })
  })

  it('probes now and updates health details', async () => {
    const user = userEvent.setup()
    mockGetEntry.mockResolvedValue(baseEntry)
    mockPostProbe.mockResolvedValue({
      state: 'degraded',
      latencyMs: 142,
      consecutiveFailures: 1,
      lastError: 'timeout',
      lastProbedAt: new Date(Date.now() - 30_000).toISOString(),
      lastSuccessAt: new Date(Date.now() - 65_000).toISOString(),
    })

    renderEntryDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /probe now/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /probe now/i }))

    await waitFor(() => {
      expect(mockPostProbe).toHaveBeenCalledWith('entry-1')
      expect(screen.getByText('142 ms')).toBeInTheDocument()
      expect(screen.getByText('timeout')).toBeInTheDocument()
      expect(screen.getByText('30s ago')).toBeInTheDocument()
      expect(screen.getByText('1m ago')).toBeInTheDocument()
    })
  })

  it('shows action error when probe now fails', async () => {
    const user = userEvent.setup()
    mockGetEntry.mockResolvedValue(baseEntry)
    mockPostProbe.mockRejectedValue(new Error('Probe failed on server'))

    renderEntryDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /probe now/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /probe now/i }))

    await waitFor(() => {
      expect(screen.getByText('Probe failed on server')).toBeInTheDocument()
    })
  })

  it('deprecates an entry through the confirmation dialog', async () => {
    const user = userEvent.setup()
    mockGetEntry.mockResolvedValue(baseEntry)
    mockPatchLifecycle.mockResolvedValue({ ...baseEntry, status: 'deprecated' })

    renderEntryDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /^deprecate$/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /^deprecate$/i }))
    await user.click(screen.getByRole('button', { name: /^deprecate$/i }))

    await waitFor(() => {
      expect(mockPatchLifecycle).toHaveBeenCalledWith('entry-1', 'deprecated')
      expect(screen.getByRole('button', { name: /un-deprecate/i })).toBeInTheDocument()
    })
  })

  it('un-deprecates an entry', async () => {
    const user = userEvent.setup()
    mockGetEntry.mockResolvedValue({ ...baseEntry, status: 'deprecated' })
    mockPatchLifecycle.mockResolvedValue({ ...baseEntry, status: 'active' })

    renderEntryDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /un-deprecate/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /un-deprecate/i }))

    await waitFor(() => {
      expect(mockPatchLifecycle).toHaveBeenCalledWith('entry-1', 'active')
      expect(screen.getByRole('button', { name: /^deprecate$/i })).toBeInTheDocument()
    })
  })

  it('shows lifecycle action error when deprecate fails', async () => {
    const user = userEvent.setup()
    mockGetEntry.mockResolvedValue(baseEntry)
    mockPatchLifecycle.mockRejectedValue(new Error('Cannot deprecate'))

    renderEntryDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /^deprecate$/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /^deprecate$/i }))
    await user.click(screen.getByRole('button', { name: /^deprecate$/i }))

    await waitFor(() => {
      expect(screen.getByText('Cannot deprecate')).toBeInTheDocument()
    })
  })

  it('disables probe now for deprecated entries', async () => {
    mockGetEntry.mockResolvedValue({ ...baseEntry, status: 'deprecated' })

    renderEntryDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /probe now/i })).toBeDisabled()
    })
  })

  it('renders hours in relative time for older health timestamps', async () => {
    mockGetEntry.mockResolvedValue({
      ...baseEntry,
      health: {
        ...baseEntry.health,
        lastProbedAt: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
      },
    })

    renderEntryDetail()

    await waitFor(() => {
      expect(screen.getByText('2h ago')).toBeInTheDocument()
    })
  })
})

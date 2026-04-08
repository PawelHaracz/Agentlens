import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, vi, beforeEach, expect } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import CatalogDetailPage from './CatalogDetailPage'
import * as api from '@/api'
import type { CatalogEntry, Health } from '../../types'

vi.mock('@/api', () => ({
  getEntry: vi.fn(),
  postProbe: vi.fn(),
  patchLifecycle: vi.fn(),
  getRawCard: vi.fn(),
}))

// Prism.js is not available in jsdom — mock it
vi.mock('prismjs', () => ({ default: { highlightAll: vi.fn() } }))
vi.mock('prismjs/components/prism-json', () => ({}))

const HEALTH: Health = { state: 'active', lastProbedAt: null, lastSuccessAt: null, latencyMs: 42, consecutiveFailures: 0, lastError: '' }

const ENTRY: CatalogEntry = {
  id: 'entry-1',
  agent_type_id: 'at-1',
  display_name: 'Test Agent',
  description: 'A test agent.',
  protocol: 'a2a',
  endpoint: 'https://test-agent.example.com',
  version: '1.0.0',
  spec_version: '1.0',
  status: 'active',
  source: 'push',
  categories: ['nlp', 'translation'],
  capabilities: [
    { kind: 'a2a.skill', name: 'translate', description: 'Translate text' },
  ],
  provider: { organization: 'ACME Corp', team: 'AI', url: 'https://acme.example.com' },
  validity: { last_seen: new Date().toISOString() },
  health: HEALTH,
  metadata: {},
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
}

const RAW_CARD_RESPONSE = {
  data: '{"name":"test"}',
  contentType: 'application/json',
  fetchedAt: new Date().toISOString(),
  truncated: false,
}

function renderPage(id = 'entry-1') {
  return render(
    <MemoryRouter initialEntries={[`/catalog/${id}`]}>
      <Routes>
        <Route path="/catalog/:id" element={<CatalogDetailPage />} />
        <Route path="/" element={<div>Home</div>} />
      </Routes>
    </MemoryRouter>
  )
}

describe('CatalogDetailPage', () => {
  beforeEach(() => {
    vi.spyOn(api, 'getEntry').mockResolvedValue(ENTRY)
    vi.spyOn(api, 'getRawCard').mockResolvedValue(RAW_CARD_RESPONSE)
    vi.spyOn(api, 'postProbe').mockResolvedValue(HEALTH)
    vi.spyOn(api, 'patchLifecycle').mockResolvedValue({ ...ENTRY, status: 'deprecated' })
  })

  it('shows loading skeleton initially', () => {
    // Don't resolve promise yet
    vi.spyOn(api, 'getEntry').mockReturnValue(new Promise(() => {}))
    renderPage()
    // Skeletons are rendered as divs with animate-pulse class
    expect(document.querySelector('.animate-pulse')).toBeTruthy()
  })

  it('renders entry details after load', async () => {
    renderPage()
    await screen.findByText('Test Agent')
    expect(screen.getByText('A test agent.')).toBeInTheDocument()
    expect(screen.getByText('https://test-agent.example.com')).toBeInTheDocument()
    expect(screen.getByText('push')).toBeInTheDocument()
    expect(screen.getByText('1.0.0')).toBeInTheDocument()
  })

  it('renders categories as chips', async () => {
    renderPage()
    await screen.findByText('Test Agent')
    expect(screen.getByText('nlp')).toBeInTheDocument()
    expect(screen.getByText('translation')).toBeInTheDocument()
  })

  it('renders capabilities list', async () => {
    renderPage()
    await screen.findByText('translate')
    expect(screen.getByText('Translate text')).toBeInTheDocument()
  })

  it('renders provider info', async () => {
    renderPage()
    await screen.findByText('ACME Corp')
  })

  it('renders health latency', async () => {
    renderPage()
    // "42 ms" appears in both the StatusBadge and the Health section
    const elements = await screen.findAllByText(/42 ms/)
    expect(elements.length).toBeGreaterThanOrEqual(1)
  })

  it('shows error state when load fails', async () => {
    vi.spyOn(api, 'getEntry').mockRejectedValue(new Error('Network error'))
    renderPage()
    await screen.findByText(/Failed to load entry/i)
    expect(screen.getByText('Network error')).toBeInTheDocument()
  })

  it('retries on Retry button click', async () => {
    vi.spyOn(api, 'getEntry').mockRejectedValueOnce(new Error('fail'))
      .mockResolvedValueOnce(ENTRY)
    renderPage()
    const retry = await screen.findByRole('button', { name: /retry/i })
    fireEvent.click(retry)
    await screen.findByText('Test Agent')
  })

  it('shows Probe Now button and calls postProbe on click', async () => {
    renderPage()
    const btn = await screen.findByRole('button', { name: /probe now/i })
    fireEvent.click(btn)
    await waitFor(() => expect(api.postProbe).toHaveBeenCalledWith('entry-1'))
  })

  it('shows Deprecate button for active entry', async () => {
    renderPage()
    await screen.findByRole('button', { name: /deprecate/i })
  })

  it('shows Undeprecate button for deprecated entry', async () => {
    vi.spyOn(api, 'getEntry').mockResolvedValue({ ...ENTRY, status: 'deprecated' })
    renderPage()
    await screen.findByRole('button', { name: /undeprecate/i })
  })

  it('calls patchLifecycle when Deprecate is clicked', async () => {
    renderPage()
    const btn = await screen.findByRole('button', { name: /deprecate/i })
    fireEvent.click(btn)
    await waitFor(() => expect(api.patchLifecycle).toHaveBeenCalledWith('entry-1', 'deprecated'))
  })

  it('shows inline action error when patchLifecycle fails', async () => {
    vi.spyOn(api, 'patchLifecycle').mockRejectedValue(new Error('permission denied'))
    renderPage()
    const btn = await screen.findByRole('button', { name: /deprecate/i })
    fireEvent.click(btn)
    await screen.findByText('permission denied')
  })

  it('renders Overview and Raw Card tabs', async () => {
    renderPage()
    await screen.findByText('Test Agent')
    expect(screen.getByRole('tab', { name: /overview/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /raw card/i })).toBeInTheDocument()
  })

  it('has Back button', async () => {
    renderPage()
    await screen.findByRole('button', { name: /back/i })
  })

  it('renders null when entry is missing after loading', async () => {
    vi.spyOn(api, 'getEntry').mockResolvedValue(null as unknown as CatalogEntry)
    const { container } = renderPage()
    // stays at loading until null resolves, then renders nothing in container
    await waitFor(() => {
      expect(container.querySelector('.animate-pulse')).toBeNull()
    })
  })

  it('entry without description omits description paragraph', async () => {
    vi.spyOn(api, 'getEntry').mockResolvedValue({ ...ENTRY, description: '' })
    renderPage()
    await screen.findByText('Test Agent')
    expect(screen.queryByText('A test agent.')).not.toBeInTheDocument()
  })

  it('entry without provider omits provider row', async () => {
    vi.spyOn(api, 'getEntry').mockResolvedValue({ ...ENTRY, provider: undefined })
    renderPage()
    await screen.findByText('Test Agent')
    expect(screen.queryByText('ACME Corp')).not.toBeInTheDocument()
  })

  it('entry with lastError shows error in health section', async () => {
    vi.spyOn(api, 'getEntry').mockResolvedValue({
      ...ENTRY,
      health: { ...ENTRY.health!, lastError: 'connection refused' },
    })
    renderPage()
    await screen.findByText('connection refused')
  })
})

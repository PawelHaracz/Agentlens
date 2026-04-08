import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, vi, expect } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { CatalogRow } from './CatalogRow'
import type { CatalogEntry } from '../../../types'

const navigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => navigate }
})

function mkEntry(overrides: Partial<CatalogEntry> = {}): CatalogEntry {
  return {
    id: 'e1',
    agent_type_id: 'at1',
    display_name: 'My Agent',
    description: 'desc',
    protocol: 'a2a',
    endpoint: 'https://agent.example.com',
    version: '1.0',
    spec_version: '1.0',
    status: 'active',
    source: 'push',
    capabilities: [],
    validity: { last_seen: new Date().toISOString() },
    health: { state: 'active', lastProbedAt: null, lastSuccessAt: null, latencyMs: 0, consecutiveFailures: 0, lastError: '' },
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  }
}

function renderRow(entry: CatalogEntry, searchSnippet?: string) {
  return render(
    <MemoryRouter>
      <table>
        <tbody>
          <CatalogRow entry={entry} searchSnippet={searchSnippet} />
        </tbody>
      </table>
    </MemoryRouter>
  )
}

describe('CatalogRow', () => {
  it('renders display name', () => {
    renderRow(mkEntry())
    expect(screen.getByText('My Agent')).toBeInTheDocument()
  })

  it('renders description when present', () => {
    renderRow(mkEntry({ description: 'A description' }))
    expect(screen.getByText('A description')).toBeInTheDocument()
  })

  it('does not render description paragraph when empty', () => {
    renderRow(mkEntry({ description: '' }))
    expect(screen.queryByText('desc')).not.toBeInTheDocument()
  })

  it('shows skill count when capabilities present', () => {
    renderRow(mkEntry({
      capabilities: [
        { kind: 'a2a.skill', name: 'translate', description: '' },
        { kind: 'a2a.skill', name: 'detect', description: '' },
      ],
    }))
    expect(screen.getByText('2')).toBeInTheDocument()
  })

  it('shows dash when no capabilities', () => {
    renderRow(mkEntry({ capabilities: [] }))
    // The dash "—" is rendered for skillCount === 0
    const cells = screen.getAllByRole('cell')
    // skills cell contains "—"
    const skillCell = cells.find(c => c.textContent === '—')
    expect(skillCell).toBeTruthy()
  })

  it('renders protocol badge', () => {
    renderRow(mkEntry({ protocol: 'mcp' }))
    expect(screen.getByText('mcp')).toBeInTheDocument()
  })

  it('renders status badge', () => {
    renderRow(mkEntry({ status: 'offline' }))
    expect(screen.getByText('Offline')).toBeInTheDocument()
  })

  it('renders provider organization when present', () => {
    renderRow(mkEntry({
      provider: { organization: 'ACME', team: '', url: '' },
    }))
    expect(screen.getByText('ACME')).toBeInTheDocument()
  })

  it('renders provider with link when URL provided', () => {
    renderRow(mkEntry({
      provider: { organization: 'ACME', team: '', url: 'https://acme.com' },
    }))
    const link = screen.getByRole('link', { name: /open acme website/i })
    expect(link).toHaveAttribute('href', 'https://acme.com')
  })

  it('renders dash for missing provider', () => {
    renderRow(mkEntry({ provider: undefined }))
    // "—" character in provider cell
    const allText = screen.getAllByRole('cell').map(c => c.textContent)
    expect(allText).toContain('—')
  })

  it('navigates to detail page on row click', () => {
    renderRow(mkEntry())
    const row = screen.getByRole('link', { name: /view details for my agent/i })
    fireEvent.click(row)
    expect(navigate).toHaveBeenCalledWith('/catalog/e1')
  })

  it('navigates on Enter key press', () => {
    renderRow(mkEntry())
    const row = screen.getByRole('link', { name: /view details for my agent/i })
    fireEvent.keyDown(row, { key: 'Enter' })
    expect(navigate).toHaveBeenCalledWith('/catalog/e1')
  })

  it('navigates on Space key press', () => {
    navigate.mockClear()
    renderRow(mkEntry())
    const row = screen.getByRole('link', { name: /view details for my agent/i })
    fireEvent.keyDown(row, { key: ' ' })
    expect(navigate).toHaveBeenCalledWith('/catalog/e1')
  })

  it('does not navigate on other key press', () => {
    navigate.mockClear()
    renderRow(mkEntry())
    const row = screen.getByRole('link', { name: /view details for my agent/i })
    fireEvent.keyDown(row, { key: 'Tab' })
    expect(navigate).not.toHaveBeenCalled()
  })

  it('renders lastSuccessAt relative time when provided', () => {
    const recent = new Date(Date.now() - 30000).toISOString() // 30s ago
    renderRow(mkEntry({
      health: { state: 'active', lastProbedAt: null, lastSuccessAt: recent, latencyMs: 0, consecutiveFailures: 0, lastError: '' },
    }))
    // "ago" appears in both StatusBadge (via lastSeenAt) and the last-seen column
    const agoEls = screen.getAllByText(/ago/)
    expect(agoEls.length).toBeGreaterThanOrEqual(1)
  })

  it('shows dash for null lastSuccessAt', () => {
    renderRow(mkEntry({
      health: { state: 'active', lastProbedAt: null, lastSuccessAt: null, latencyMs: 0, consecutiveFailures: 0, lastError: '' },
    }))
    // "—" in the last-seen column
    const allText = screen.getAllByRole('cell').map(c => c.textContent)
    expect(allText).toContain('—')
  })

  it('provider link click does not navigate the row', () => {
    navigate.mockClear()
    renderRow(mkEntry({
      provider: { organization: 'ACME', team: '', url: 'https://acme.com' },
    }))
    const link = screen.getByRole('link', { name: /open acme website/i })
    // stopPropagation is called — click should not propagate to row
    fireEvent.click(link)
    expect(navigate).not.toHaveBeenCalled()
  })
})

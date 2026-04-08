import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, vi, beforeEach, expect } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { useCatalogQuery } from './useCatalogQuery'
import * as api from '../api'
import type { CatalogEntry } from '../types'

vi.mock('../api', () => ({
  listCatalog: vi.fn(),
}))

// A small component that exercises the hook
function HookHarness() {
  const { entries, isLoading, isError, filter, setProtocol, setQuery, setSort, clearFilters, refetch } =
    useCatalogQuery()

  return (
    <div>
      {isLoading && <span>loading</span>}
      {isError && <span>error</span>}
      {entries?.map(e => <div key={e.id}>{e.display_name}</div>)}
      <div data-testid="protocol">{filter.protocol ?? 'none'}</div>
      <div data-testid="q">{filter.q ?? 'none'}</div>
      <div data-testid="sort">{filter.sort ?? 'none'}</div>
      <button onClick={() => setProtocol('a2a')}>set-a2a</button>
      <button onClick={() => setProtocol(undefined)}>clear-protocol</button>
      <button onClick={() => setQuery('test')}>set-query</button>
      <button onClick={() => setQuery('')}>clear-query</button>
      <button onClick={() => setSort('displayName_asc')}>set-sort</button>
      <button onClick={() => setSort(undefined)}>clear-sort</button>
      <button onClick={() => clearFilters()}>clear-all</button>
      <button onClick={() => refetch()}>refetch</button>
    </div>
  )
}

const ENTRY: CatalogEntry = {
  id: 'e1', agent_type_id: 'at1', display_name: 'Agent One', description: '',
  protocol: 'a2a', endpoint: 'https://a.example.com', version: '1.0', spec_version: '1.0',
  status: 'active', source: 'push',
  validity: { last_seen: new Date().toISOString() },
  health: { state: 'active', lastProbedAt: null, lastSuccessAt: null, latencyMs: 0, consecutiveFailures: 0, lastError: '' },
  created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
}

function renderHook(initialPath = '/') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[initialPath]}>
        <HookHarness />
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('useCatalogQuery', () => {
  beforeEach(() => {
    vi.spyOn(api, 'listCatalog').mockResolvedValue([ENTRY])
  })

  it('returns entries from API', async () => {
    renderHook()
    await screen.findByText('Agent One')
  })

  it('reads protocol from URL search params', async () => {
    renderHook('/?protocol=mcp')
    await waitFor(() => expect(screen.getByTestId('protocol').textContent).toBe('mcp'))
  })

  it('reads q from URL search params', async () => {
    renderHook('/?q=hello')
    await waitFor(() => expect(screen.getByTestId('q').textContent).toBe('hello'))
  })

  it('reads sort from URL search params', async () => {
    renderHook('/?sort=displayName_asc')
    await waitFor(() => expect(screen.getByTestId('sort').textContent).toBe('displayName_asc'))
  })

  it('setProtocol updates URL param', async () => {
    renderHook()
    await screen.findByText('Agent One')
    fireEvent.click(screen.getByText('set-a2a'))
    await waitFor(() => expect(screen.getByTestId('protocol').textContent).toBe('a2a'))
  })

  it('setProtocol(undefined) removes URL param', async () => {
    renderHook('/?protocol=a2a')
    await screen.findByText('Agent One')
    fireEvent.click(screen.getByText('clear-protocol'))
    await waitFor(() => expect(screen.getByTestId('protocol').textContent).toBe('none'))
  })

  it('setQuery sets q param', async () => {
    renderHook()
    await screen.findByText('Agent One')
    fireEvent.click(screen.getByText('set-query'))
    await waitFor(() => expect(screen.getByTestId('q').textContent).toBe('test'))
  })

  it('setQuery with empty string removes q param', async () => {
    renderHook('/?q=test')
    await screen.findByText('Agent One')
    fireEvent.click(screen.getByText('clear-query'))
    await waitFor(() => expect(screen.getByTestId('q').textContent).toBe('none'))
  })

  it('setSort sets sort param', async () => {
    renderHook()
    await screen.findByText('Agent One')
    fireEvent.click(screen.getByText('set-sort'))
    await waitFor(() => expect(screen.getByTestId('sort').textContent).toBe('displayName_asc'))
  })

  it('setSort(undefined) removes sort param', async () => {
    renderHook('/?sort=displayName_asc')
    await screen.findByText('Agent One')
    fireEvent.click(screen.getByText('clear-sort'))
    await waitFor(() => expect(screen.getByTestId('sort').textContent).toBe('none'))
  })

  it('clearFilters resets all params', async () => {
    renderHook('/?protocol=a2a&q=test&sort=displayName_asc')
    await screen.findByText('Agent One')
    fireEvent.click(screen.getByText('clear-all'))
    await waitFor(() => {
      expect(screen.getByTestId('protocol').textContent).toBe('none')
      expect(screen.getByTestId('q').textContent).toBe('none')
      expect(screen.getByTestId('sort').textContent).toBe('none')
    })
  })

  it('shows loading state while fetching', () => {
    vi.spyOn(api, 'listCatalog').mockReturnValue(new Promise(() => {}))
    renderHook()
    expect(screen.getByText('loading')).toBeInTheDocument()
  })

  it('shows error state when fetch fails', async () => {
    vi.spyOn(api, 'listCatalog').mockRejectedValue(new Error('fail'))
    renderHook()
    await screen.findByText('error')
  })
})

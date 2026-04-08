import { render, screen } from '@testing-library/react'
import { describe, it, vi, beforeEach } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import React from 'react'
import CatalogListPage from './CatalogListPage'
import * as api from '../../api'

vi.mock('../../api', () => ({
  listCatalog: vi.fn(),
  getStats: vi.fn(),
}))

const wrapper = ({ children }: { children: React.ReactNode }) =>
  React.createElement(
    QueryClientProvider,
    { client: new QueryClient({ defaultOptions: { queries: { retry: false } } }) },
    React.createElement(MemoryRouter, null, children)
  )

describe('CatalogListPage', () => {
  beforeEach(() => {
    vi.spyOn(api, 'listCatalog').mockResolvedValue([])
    vi.spyOn(api, 'getStats').mockResolvedValue({ total: 0, by_status: {}, by_source: {} })
  })

  it('shows empty state when no entries', async () => {
    render(<CatalogListPage />, { wrapper })
    await screen.findByText(/No agents registered yet/i)
  })

  it('renders table row when entries present', async () => {
    vi.spyOn(api, 'listCatalog').mockResolvedValue([
      {
        id: 'e1', display_name: 'My Agent', description: '', protocol: 'a2a' as const,
        endpoint: 'http://agent.test', version: '1', status: 'active' as const, source: 'push' as const,
        agent_type_id: 'at1',
        validity: { last_seen: new Date().toISOString() },
        health: { state: 'active' as const, lastProbedAt: null, lastSuccessAt: null, latencyMs: 0, consecutiveFailures: 0, lastError: '' },
        created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
      },
    ])
    render(<CatalogListPage />, { wrapper })
    await screen.findByText('My Agent')
  })
})

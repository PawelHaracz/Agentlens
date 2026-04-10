import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, vi, beforeEach, expect } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import React from 'react'
import CapabilityListPage from './CapabilityListPage'
import * as api from '@/api'
import type { CapabilityListResult } from '@/types'

vi.mock('@/api', () => ({
  listCapabilities: vi.fn(),
}))

const ITEMS_A2A: CapabilityListResult = {
  total: 2,
  items: [
    {
      kind: 'a2a.skill',
      name: 'translate',
      description: 'Translate text between languages',
      tags: ['nlp', 'translation'],
      input_modes: null,
      output_modes: null,
      agent_id: 'agent-1',
      agent_name: 'Translator Agent',
      protocol: 'a2a',
      status: 'active',
      spec_version: '1.0',
      provider_org: 'ACME',
      provider_url: null,
      health_state: 'active',
      latency_ms: 42,
    },
    {
      kind: 'mcp.tool',
      name: 'search',
      description: 'Search for documents',
      tags: null,
      input_modes: null,
      output_modes: null,
      agent_id: 'agent-2',
      agent_name: 'Search Agent',
      protocol: 'mcp',
      status: 'active',
      spec_version: '1.0',
      provider_org: null,
      provider_url: null,
      health_state: 'active',
      latency_ms: 10,
    },
  ],
}

const wrapper = ({ children }: { children: React.ReactNode }) =>
  React.createElement(
    QueryClientProvider,
    { client: new QueryClient({ defaultOptions: { queries: { retry: false } } }) },
    React.createElement(MemoryRouter, null, children)
  )

describe('CapabilityListPage', () => {
  beforeEach(() => {
    vi.spyOn(api, 'listCapabilities').mockResolvedValue({ total: 0, items: [] })
  })

  it('shows loading spinner initially', () => {
    vi.spyOn(api, 'listCapabilities').mockReturnValue(new Promise(() => {}))
    render(<CapabilityListPage />, { wrapper })
    expect(document.querySelector('.animate-spin')).toBeTruthy()
  })

  it('shows empty state when no capabilities', async () => {
    render(<CapabilityListPage />, { wrapper })
    await screen.findByText(/No capabilities published yet/i)
  })

  it('renders capability groups when data is present', async () => {
    vi.spyOn(api, 'listCapabilities').mockResolvedValue(ITEMS_A2A)
    render(<CapabilityListPage />, { wrapper })
    await screen.findByText('translate')
    expect(screen.getByText('search')).toBeInTheDocument()
  })

  it('shows error state when load fails', async () => {
    vi.spyOn(api, 'listCapabilities').mockRejectedValue(new Error('Network error'))
    render(<CapabilityListPage />, { wrapper })
    await screen.findByText(/Failed to load capabilities/i)
    expect(screen.getByText('Network error')).toBeInTheDocument()
  })

  it('shows filter empty state and clear button when query active but no results', async () => {
    // Render with a q param so hasActiveFilters=true
    const wrapperWithQuery = ({ children }: { children: React.ReactNode }) =>
      React.createElement(
        QueryClientProvider,
        { client: new QueryClient({ defaultOptions: { queries: { retry: false } } }) },
        React.createElement(MemoryRouter, { initialEntries: ['/?q=xyz'] }, children)
      )
    render(<CapabilityListPage />, { wrapper: wrapperWithQuery })
    await screen.findByText(/No capabilities match the current filters/i)
    expect(screen.getByRole('button', { name: /clear filters/i })).toBeInTheDocument()
  })

  it('clear filters button resets search params', async () => {
    const wrapperWithQuery = ({ children }: { children: React.ReactNode }) =>
      React.createElement(
        QueryClientProvider,
        { client: new QueryClient({ defaultOptions: { queries: { retry: false } } }) },
        React.createElement(MemoryRouter, { initialEntries: ['/?q=xyz'] }, children)
      )
    render(<CapabilityListPage />, { wrapper: wrapperWithQuery })
    const clearBtn = await screen.findByRole('button', { name: /clear filters/i })
    fireEvent.click(clearBtn)
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /clear filters/i })).not.toBeInTheDocument()
    })
  })

  it('renders page header', async () => {
    render(<CapabilityListPage />, { wrapper })
    await screen.findByText('Capabilities')
    expect(screen.getByText(/Discover agents by capability/i)).toBeInTheDocument()
  })
})

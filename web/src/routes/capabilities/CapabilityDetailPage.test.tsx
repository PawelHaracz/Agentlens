import { render, screen, waitFor } from '@testing-library/react'
import { describe, it, vi, beforeEach, expect } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import React from 'react'
import CapabilityDetailPage from './CapabilityDetailPage'
import * as api from '@/api'
import type { CapabilityDetailResponse } from '@/types'

vi.mock('@/api', () => ({
  getCapabilityAgents: vi.fn(),
}))

const DETAIL: CapabilityDetailResponse = {
  capability: { kind: 'a2a.skill', name: 'translate' },
  agents: [
    {
      id: 'agent-1',
      display_name: 'Translator Agent',
      protocol: 'a2a',
      provider: { organization: 'ACME Corp', url: 'https://acme.example.com' },
      health: { state: 'active', latencyMs: 42 },
      spec_version: '1.0',
      status: 'active',
      capability_snippet: { description: 'Translate text between languages' },
    },
  ],
}

function renderPage(key = encodeURIComponent('a2a.skill::translate')) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    React.createElement(
      QueryClientProvider,
      { client: qc },
      React.createElement(
        MemoryRouter,
        { initialEntries: [`/catalog/capabilities/${key}`] },
        React.createElement(
          Routes,
          null,
          React.createElement(Route, {
            path: '/catalog/capabilities/:key',
            element: React.createElement(CapabilityDetailPage),
          })
        )
      )
    )
  )
}

describe('CapabilityDetailPage', () => {
  beforeEach(() => {
    vi.spyOn(api, 'getCapabilityAgents').mockResolvedValue(DETAIL)
  })

  it('shows loading spinner initially', () => {
    vi.spyOn(api, 'getCapabilityAgents').mockReturnValue(new Promise(() => {}))
    renderPage()
    expect(document.querySelector('.animate-spin')).toBeTruthy()
  })

  it('renders agent name after load', async () => {
    renderPage()
    await screen.findByText('Translator Agent')
  })

  it('renders capability name as heading', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'translate' })
  })

  it('renders A2A Skill kind badge', async () => {
    renderPage()
    await screen.findByText('A2A Skill')
  })

  it('renders agent count', async () => {
    renderPage()
    await screen.findByText(/1 agent$/)
  })

  it('renders provider organization', async () => {
    renderPage()
    await screen.findByText('ACME Corp')
  })

  it('shows error state when load fails', async () => {
    vi.spyOn(api, 'getCapabilityAgents').mockRejectedValue(new Error('fetch error'))
    renderPage()
    await screen.findByText(/Failed to load capability details/i)
    expect(screen.getByText('fetch error')).toBeInTheDocument()
  })

  it('shows empty state when no agents', async () => {
    vi.spyOn(api, 'getCapabilityAgents').mockResolvedValue({
      capability: { kind: 'a2a.skill', name: 'translate' },
      agents: [],
    })
    renderPage()
    await screen.findByText(/No agents offer this capability/i)
  })

  it('renders table headers', async () => {
    renderPage()
    await screen.findByText('Agent')
    expect(screen.getByText('Protocol')).toBeInTheDocument()
    expect(screen.getByText('Status')).toBeInTheDocument()
  })

  it('renders Back to Capabilities link', async () => {
    renderPage()
    await screen.findByText(/Back to Capabilities/i)
  })

  it('shows invalid key message when key param is missing', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      React.createElement(
        QueryClientProvider,
        { client: qc },
        React.createElement(
          MemoryRouter,
          { initialEntries: ['/catalog/capabilities/'] },
          React.createElement(
            Routes,
            null,
            React.createElement(Route, {
              path: '/catalog/capabilities/',
              element: React.createElement(CapabilityDetailPage),
            })
          )
        )
      )
    )
    await waitFor(() => {
      expect(screen.getByText(/Invalid capability key/i)).toBeInTheDocument()
    })
  })

  it('handles unknown kind gracefully', async () => {
    vi.spyOn(api, 'getCapabilityAgents').mockResolvedValue({
      ...DETAIL,
      capability: { kind: 'custom.kind', name: 'do-thing' },
      agents: [{ ...DETAIL.agents[0] }],
    })
    renderPage(encodeURIComponent('custom.kind::do-thing'))
    // falls back to the raw kind string
    await screen.findByText('custom.kind')
  })
})

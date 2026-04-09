import { renderHook, waitFor } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useCapabilitiesQuery } from './useCapabilitiesQuery'

// Mock API
vi.mock('../api', () => ({
  listCapabilities: vi.fn(() =>
    Promise.resolve({
      total: 1,
      items: [
        {
          kind: 'a2a.skill',
          name: 'Test Skill',
          description: 'Test',
          tags: null,
          input_modes: null,
          output_modes: null,
          agent_id: 'test-id',
          agent_name: 'Test Agent',
          protocol: 'a2a',
          status: 'active',
          spec_version: '1.0',
          provider_org: null,
          provider_url: null,
          health_state: 'active',
          latency_ms: 100,
        },
      ],
    })
  ),
}))

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return ({ children }: { children: React.ReactNode }) => (
    React.createElement(BrowserRouter, null,
      React.createElement(QueryClientProvider, { client: queryClient }, children)
    )
  )
}

describe('useCapabilitiesQuery', () => {
  it('syncs query param to URL', async () => {
    const wrapper = createWrapper()
    const { result } = renderHook(() => useCapabilitiesQuery(), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    // setQuery updates URL
    result.current.setQuery('test')
    await waitFor(() => {
      const url = new URL(window.location.href)
      expect(url.searchParams.get('q')).toBe('test')
    })
  })

  it('syncs kind param to URL', async () => {
    const wrapper = createWrapper()
    const { result } = renderHook(() => useCapabilitiesQuery(), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    result.current.setKind('a2a.skill')
    await waitFor(() => {
      const url = new URL(window.location.href)
      expect(url.searchParams.get('kind')).toBe('a2a.skill')
    })
  })
})

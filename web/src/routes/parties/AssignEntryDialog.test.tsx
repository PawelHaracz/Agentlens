import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AssignEntryDialog from './AssignEntryDialog'

vi.mock('@/api', () => ({
  listCatalog: vi.fn(),
  assignEntryToProject: vi.fn(),
}))
import * as api from '@/api'
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

const makeEntry = (id: string, name: string) => ({
  id,
  display_name: name,
  description: '',
  protocol: 'a2a' as const,
  endpoint: `https://${id}.example`,
  version: '1.0',
  status: 'active' as const,
  source: 'push' as const,
  agent_type_id: 't',
  validity: { last_seen: '' },
  health: { state: 'active' as const, latencyMs: 0, consecutiveFailures: 0, lastError: '' },
  created_at: '',
  updated_at: '',
})

beforeEach(() => {
  vi.clearAllMocks()
  mockApi.listCatalog.mockResolvedValue([])
})

describe('AssignEntryDialog', () => {
  it('searches and shows results', async () => {
    mockApi.listCatalog.mockImplementation(async ({ q }: { q?: string }) => {
      if (!q) return []
      return [makeEntry('e1', `Found ${q}`)]
    })
    render(<AssignEntryDialog open={true} onOpenChange={vi.fn()} onAssigned={vi.fn()} projectId="p1" alreadyAssignedIds={new Set()} />)
    await userEvent.type(screen.getByPlaceholderText(/search catalog/i), 'trans')
    await waitFor(() => expect(screen.getByText('Found trans')).toBeInTheDocument(), { timeout: 2000 })
  })

  it('filters out already-assigned entries', async () => {
    mockApi.listCatalog.mockImplementation(async () => [
      makeEntry('e1', 'Alpha'),
      makeEntry('e2', 'Bravo'),
    ])
    render(<AssignEntryDialog open={true} onOpenChange={vi.fn()} onAssigned={vi.fn()} projectId="p1" alreadyAssignedIds={new Set(['e1'])} />)
    await userEvent.type(screen.getByPlaceholderText(/search catalog/i), 'a')
    await waitFor(() => expect(screen.getByText('Bravo')).toBeInTheDocument(), { timeout: 2000 })
    expect(screen.queryByText('Alpha')).not.toBeInTheDocument()
  })

  it('assigns on confirm', async () => {
    mockApi.listCatalog.mockResolvedValue([makeEntry('e1', 'Alpha')])
    mockApi.assignEntryToProject.mockResolvedValue(undefined)
    const onAssigned = vi.fn()
    render(<AssignEntryDialog open={true} onOpenChange={vi.fn()} onAssigned={onAssigned} projectId="p1" alreadyAssignedIds={new Set()} />)
    await userEvent.type(screen.getByPlaceholderText(/search catalog/i), 'a')
    await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument(), { timeout: 2000 })
    await userEvent.click(screen.getByText('Alpha'))
    await userEvent.click(screen.getByRole('button', { name: /^assign$/i }))
    await waitFor(() => expect(mockApi.assignEntryToProject).toHaveBeenCalledWith('e1', 'p1'))
    expect(onAssigned).toHaveBeenCalled()
  })
})

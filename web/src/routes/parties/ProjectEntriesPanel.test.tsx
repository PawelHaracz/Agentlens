import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ProjectEntriesPanel from './ProjectEntriesPanel'

vi.mock('@/api', () => ({
  listCatalog: vi.fn(),
  removeEntryFromProject: vi.fn(),
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

describe('ProjectEntriesPanel', () => {
  it('renders heading and empty state', async () => {
    render(<ProjectEntriesPanel projectId="p1" canWrite={true} />)
    expect(screen.getByRole('heading', { name: /assigned catalog entries/i })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText(/no entries assigned/i)).toBeInTheDocument())
  })

  it('renders rows for each assigned entry', async () => {
    mockApi.listCatalog.mockResolvedValue([makeEntry('e1', 'Alpha')])
    render(<ProjectEntriesPanel projectId="p1" canWrite={true} />)
    await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument())
  })

  it('hides Assign button without canWrite', () => {
    render(<ProjectEntriesPanel projectId="p1" canWrite={false} />)
    expect(screen.queryByRole('button', { name: /assign entry/i })).not.toBeInTheDocument()
  })

  it('removes assigned entry on confirm', async () => {
    mockApi.listCatalog
      .mockResolvedValueOnce([makeEntry('e1', 'Alpha')])
      .mockResolvedValueOnce([])
    mockApi.removeEntryFromProject.mockResolvedValue(undefined)
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<ProjectEntriesPanel projectId="p1" canWrite={true} />)
    await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument())
    await userEvent.click(screen.getByTitle('Remove'))
    await waitFor(() => expect(mockApi.removeEntryFromProject).toHaveBeenCalledWith('e1', 'p1'))
    confirmSpy.mockRestore()
  })
})

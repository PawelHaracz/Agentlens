import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import GroupsTab from './GroupsTab'

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}))
vi.mock('@/api', () => ({
  listGroups: vi.fn(),
  listGroupMembers: vi.fn(),
  createGroup: vi.fn(),
  deleteGroup: vi.fn(),
}))

import { useAuth } from '../../contexts/AuthContext'
import * as api from '@/api'

const mockUseAuth = useAuth as ReturnType<typeof vi.fn>
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => {
  vi.clearAllMocks()
  mockUseAuth.mockReturnValue({
    hasPermission: (p: string) => p === 'users:write',
  })
  mockApi.listGroups.mockResolvedValue([])
})

function renderTab() {
  return render(<MemoryRouter><GroupsTab /></MemoryRouter>)
}

describe('GroupsTab', () => {
  it('shows empty state when no groups exist', async () => {
    renderTab()
    await waitFor(() => expect(screen.getByText(/no groups yet/i)).toBeInTheDocument())
  })

  it('renders a row per group', async () => {
    mockApi.listGroups.mockResolvedValue([
      { id: 'g1', kind: 'group', name: 'platform', is_system: false, created_at: '2026-01-01T00:00:00Z', updated_at: '' },
      { id: 'g2', kind: 'group', name: 'sre', is_system: false, created_at: '2026-01-02T00:00:00Z', updated_at: '' },
    ])
    renderTab()
    await waitFor(() => expect(screen.getByText('platform')).toBeInTheDocument())
    expect(screen.getByText('sre')).toBeInTheDocument()
  })

  it('opens Create dialog when Create button is clicked', async () => {
    renderTab()
    await userEvent.click(screen.getByRole('button', { name: /create group/i }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('hides Create button for users without users:write', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: () => false })
    renderTab()
    await waitFor(() => expect(screen.queryByRole('button', { name: /create group/i })).not.toBeInTheDocument())
  })

  it('calls deleteGroup and reloads on confirmed delete', async () => {
    mockApi.listGroups
      .mockResolvedValueOnce([{ id: 'g1', kind: 'group', name: 'platform', is_system: false, created_at: '', updated_at: '' }])
      .mockResolvedValueOnce([])
    mockApi.deleteGroup.mockResolvedValue(undefined)
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderTab()
    await waitFor(() => expect(screen.getByText('platform')).toBeInTheDocument())
    await userEvent.click(screen.getByTitle('Delete'))
    await waitFor(() => expect(api.deleteGroup).toHaveBeenCalledWith('g1'))
    confirmSpy.mockRestore()
  })
})

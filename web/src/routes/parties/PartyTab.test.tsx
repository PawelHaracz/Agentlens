import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import PartyTab from './PartyTab'
import { groupUIConfig, projectUIConfig, type PartyUIConfig } from './partyUIConfig'

vi.mock('../../contexts/AuthContext', () => ({ useAuth: vi.fn() }))
vi.mock('@/api', () => ({
  listGroups: vi.fn(),
  createGroup: vi.fn(),
  deleteGroup: vi.fn(),
  listProjects: vi.fn(),
  createProject: vi.fn(),
  deleteProject: vi.fn(),
}))

import { useAuth } from '../../contexts/AuthContext'
import * as api from '@/api'

const mockUseAuth = useAuth as ReturnType<typeof vi.fn>
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

function makeRender(config: PartyUIConfig) {
  return () => render(<MemoryRouter><PartyTab config={config} /></MemoryRouter>)
}

describe.each([
  ['groups', groupUIConfig, 'listGroups', 'deleteGroup', 'Group'],
  ['projects', projectUIConfig, 'listProjects', 'deleteProject', 'Project'],
] as const)('PartyTab (%s)', (_name, config, listFn, deleteFn, label) => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p === config.writePermission })
    mockApi[listFn].mockResolvedValue([])
  })

  it('shows empty state when none exist', async () => {
    makeRender(config)()
    await waitFor(() =>
      expect(screen.getByText(new RegExp(`no ${label.toLowerCase()}s yet`, 'i'))).toBeInTheDocument()
    )
  })

  it('renders rows', async () => {
    mockApi[listFn].mockResolvedValue([
      { id: 'a', kind: config.kind, name: 'alpha', is_system: false, created_at: '2026-01-01T00:00:00Z', updated_at: '' },
    ])
    makeRender(config)()
    await waitFor(() => expect(screen.getByText('alpha')).toBeInTheDocument())
  })

  it('opens create dialog', async () => {
    makeRender(config)()
    await userEvent.click(screen.getByRole('button', { name: new RegExp(`create ${label.toLowerCase()}`, 'i') }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('hides create button without write permission', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: () => false })
    makeRender(config)()
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: new RegExp(`create ${label.toLowerCase()}`, 'i') })).not.toBeInTheDocument()
    )
  })

  it('deletes on confirmed delete', async () => {
    mockApi[listFn]
      .mockResolvedValueOnce([{ id: 'a', kind: config.kind, name: 'alpha', is_system: false, created_at: '', updated_at: '' }])
      .mockResolvedValueOnce([])
    mockApi[deleteFn].mockResolvedValue(undefined)
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    makeRender(config)()
    await waitFor(() => expect(screen.getByText('alpha')).toBeInTheDocument())
    await userEvent.click(screen.getByTitle('Delete'))
    await waitFor(() => expect(mockApi[deleteFn]).toHaveBeenCalledWith('a'))
    confirmSpy.mockRestore()
  })
})

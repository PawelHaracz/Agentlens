import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import PartyDetailPage from './PartyDetailPage'
import { groupUIConfig, projectUIConfig, type PartyUIConfig } from './partyUIConfig'

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}))
vi.mock('@/api', () => ({
  getGroup: vi.fn(),
  listGroupMembers: vi.fn(),
  addGroupMember: vi.fn(),
  removeGroupMember: vi.fn(),
  getProject: vi.fn(),
  listProjectMembers: vi.fn(),
  addProjectMember: vi.fn(),
  removeProjectMember: vi.fn(),
  updateProjectMemberRole: vi.fn(),
  listParties: vi.fn(),
  listCatalog: vi.fn().mockResolvedValue([]),
  assignEntryToProject: vi.fn(),
  removeEntryFromProject: vi.fn(),
}))

import { useAuth } from '../../contexts/AuthContext'
import * as api from '@/api'

const mockUseAuth = useAuth as ReturnType<typeof vi.fn>
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

function renderDetail({ config = groupUIConfig, id = 'g1' }: { config?: PartyUIConfig; id?: string } = {}) {
  const route = config.detailPath(id)
  return render(
    <MemoryRouter initialEntries={[route]}>
      <Routes>
        <Route path={`/settings/${config.urlPrefix}/:id`} element={<PartyDetailPage config={config} />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('GroupDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p === 'users:write' })
    mockApi.getGroup.mockResolvedValue({ id: 'g1', kind: 'group', name: 'platform', is_system: false, created_at: '2026-01-01T00:00:00Z', updated_at: '' })
    mockApi.listGroupMembers.mockResolvedValue([])
    mockApi.listParties.mockResolvedValue([])
  })

  it('renders the group name in the header', async () => {
    renderDetail()
    await waitFor(() => expect(screen.getByRole('heading', { name: /platform/i })).toBeInTheDocument())
  })

  it('renders a Back to Settings link', async () => {
    renderDetail()
    await waitFor(() => expect(screen.getByRole('link', { name: /back to settings/i })).toBeInTheDocument())
  })

  it('shows group-not-found state on 404', async () => {
    mockApi.getGroup.mockRejectedValue(new Error('not found'))
    renderDetail({ id: 'missing' })
    await waitFor(() => expect(screen.getByText(/group not found/i)).toBeInTheDocument())
  })

  it('renders member rows joining relationships with parties cache', async () => {
    mockApi.listGroupMembers.mockResolvedValue([
      { id: 'r1', from_party_id: 'p1', from_role: 'member', to_party_id: 'g1', to_role: 'group', relationship_name: 'group_member' },
      { id: 'r2', from_party_id: 'p2', from_role: 'member', to_party_id: 'g1', to_role: 'group', relationship_name: 'group_member' },
    ])
    mockApi.listParties.mockResolvedValue([
      { id: 'p1', kind: 'person', name: 'alice', is_system: false, created_at: '', updated_at: '' },
      { id: 'p2', kind: 'group', name: 'sub-team', is_system: false, created_at: '', updated_at: '' },
    ])
    renderDetail()
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    expect(screen.getByText('sub-team')).toBeInTheDocument()
    expect(screen.getByText(/^Person$/)).toBeInTheDocument()
    expect(screen.getByText(/^Group$/)).toBeInTheDocument()
  })

  it('shows empty members state', async () => {
    renderDetail()
    await waitFor(() => expect(screen.getByText(/no members yet/i)).toBeInTheDocument())
  })

  it('hides Add member button without users:write', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: () => false })
    renderDetail()
    await waitFor(() => expect(screen.queryByRole('button', { name: /add member/i })).not.toBeInTheDocument())
  })

  it('opens Add member dialog when the button is clicked', async () => {
    renderDetail()
    await waitFor(() => expect(screen.getByRole('button', { name: /add member/i })).toBeInTheDocument())
    await userEvent.click(screen.getByRole('button', { name: /add member/i }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('removes a member when Remove is clicked and confirmed', async () => {
    mockApi.listGroupMembers
      .mockResolvedValueOnce([
        { id: 'r1', from_party_id: 'p1', from_role: 'member', to_party_id: 'g1', to_role: 'group', relationship_name: 'group_member' },
      ])
      .mockResolvedValueOnce([])
    mockApi.listParties.mockResolvedValue([
      { id: 'p1', kind: 'person', name: 'alice', is_system: false, created_at: '', updated_at: '' },
    ])
    mockApi.removeGroupMember.mockResolvedValue(undefined)
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderDetail()
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    await userEvent.click(screen.getByTitle('Remove'))
    await waitFor(() => expect(api.removeGroupMember).toHaveBeenCalledWith('g1', 'p1'))
    confirmSpy.mockRestore()
  })
})

describe('PartyDetailPage (project config)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p === 'catalog:write' })
    mockApi.getProject.mockResolvedValue({ id: 'proj1', kind: 'project', name: 'demo-project', is_system: false, created_at: '2026-01-01T00:00:00Z', updated_at: '' })
    mockApi.listProjectMembers.mockResolvedValue([])
    mockApi.listParties.mockResolvedValue([])
    mockApi.listCatalog.mockResolvedValue([])
  })

  it('renders project name and entries panel', async () => {
    renderDetail({ config: projectUIConfig, id: 'proj1' })
    await waitFor(() => expect(screen.getByRole('heading', { name: /demo-project/i })).toBeInTheDocument())
    expect(screen.getByRole('heading', { name: /assigned catalog entries/i })).toBeInTheDocument()
  })

  it('renders a Role column when member rows exist', async () => {
    mockApi.listProjectMembers.mockResolvedValue([
      { id: 'r1', from_party_id: 'p1', from_role: 'project:developer', to_party_id: 'proj1', to_role: 'project', relationship_name: 'project_member' },
    ])
    mockApi.listParties.mockResolvedValue([
      { id: 'p1', kind: 'person', name: 'alice', is_system: false, created_at: '', updated_at: '' },
    ])
    renderDetail({ config: projectUIConfig, id: 'proj1' })
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    expect(screen.getByText('project:developer')).toBeInTheDocument()
  })
})

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import EditMemberRoleDialog from './EditMemberRoleDialog'

vi.mock('@/api', () => ({ updateProjectMemberRole: vi.fn() }))
import * as api from '@/api'
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => vi.clearAllMocks())

describe('EditMemberRoleDialog', () => {
  it('pre-selects the current role', () => {
    render(<EditMemberRoleDialog
      open={true} onOpenChange={vi.fn()} onSaved={vi.fn()}
      projectId="p1" memberPartyId="m1" memberName="alice"
      currentRole="project:viewer"
      roleOptions={['project:owner', 'project:developer', 'project:viewer']}
    />)
    expect((screen.getByLabelText(/role/i) as HTMLSelectElement).value).toBe('project:viewer')
  })

  it('calls updateProjectMemberRole on save', async () => {
    mockApi.updateProjectMemberRole.mockResolvedValue(undefined)
    const onSaved = vi.fn()
    render(<EditMemberRoleDialog
      open={true} onOpenChange={vi.fn()} onSaved={onSaved}
      projectId="p1" memberPartyId="m1" memberName="alice"
      currentRole="project:viewer"
      roleOptions={['project:owner', 'project:developer', 'project:viewer']}
    />)
    await userEvent.selectOptions(screen.getByLabelText(/role/i), 'project:developer')
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))
    await waitFor(() => expect(mockApi.updateProjectMemberRole).toHaveBeenCalledWith('p1', 'm1', 'project:developer'))
    expect(onSaved).toHaveBeenCalled()
  })

  it('displays backend error', async () => {
    mockApi.updateProjectMemberRole.mockRejectedValue(new Error('forbidden'))
    render(<EditMemberRoleDialog
      open={true} onOpenChange={vi.fn()} onSaved={vi.fn()}
      projectId="p1" memberPartyId="m1" memberName="alice"
      currentRole="project:viewer"
      roleOptions={['project:owner', 'project:developer', 'project:viewer']}
    />)
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))
    await waitFor(() => expect(screen.getByText(/forbidden/i)).toBeInTheDocument())
  })
})

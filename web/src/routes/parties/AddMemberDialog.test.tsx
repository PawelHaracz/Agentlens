import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AddMemberDialog from './AddMemberDialog'

vi.mock('@/api', () => ({
  addGroupMember: vi.fn(),
  addProjectMember: vi.fn(),
}))

import * as api from '@/api'
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => vi.clearAllMocks())

const parties = [
  { id: 'p1', kind: 'person' as const, name: 'alice', is_system: false, created_at: '', updated_at: '' },
  { id: 'p2', kind: 'person' as const, name: 'bob', is_system: false, created_at: '', updated_at: '' },
  { id: 'g-self', kind: 'group' as const, name: 'self', is_system: false, created_at: '', updated_at: '' },
  { id: 'g-anc', kind: 'group' as const, name: 'ancestor', is_system: false, created_at: '', updated_at: '' },
]

function renderDialog(overrides = {}) {
  return render(<AddMemberDialog
    open={true}
    onOpenChange={vi.fn()}
    onAdded={vi.fn()}
    groupId="g-self"
    parties={parties}
    excludedIds={new Set(['g-self', 'g-anc'])}
    existingMemberIds={new Set(['p1'])}
    {...overrides}
  />)
}

describe('AddMemberDialog', () => {
  it('filters out excluded and existing parties from the picker', () => {
    renderDialog()
    const options = screen.getAllByRole('option')
    const labels = options.map(o => o.textContent)
    expect(labels.some(l => l?.includes('bob'))).toBe(true)
    expect(labels.some(l => l?.includes('alice'))).toBe(false)
    expect(labels.some(l => l?.includes('self'))).toBe(false)
    expect(labels.some(l => l?.includes('ancestor'))).toBe(false)
  })

  it('calls addGroupMember on confirm', async () => {
    mockApi.addGroupMember.mockResolvedValue(undefined)
    const onAdded = vi.fn()
    renderDialog({ onAdded })
    await userEvent.selectOptions(screen.getByRole('combobox'), 'p2')
    await userEvent.click(screen.getByRole('button', { name: /^add$/i }))
    await waitFor(() => expect(api.addGroupMember).toHaveBeenCalledWith('g-self', 'p2'))
    expect(onAdded).toHaveBeenCalled()
  })

  it('translates backend cycle error to friendly message', async () => {
    mockApi.addGroupMember.mockRejectedValue(new Error('adding x → y would create a cycle'))
    renderDialog()
    await userEvent.selectOptions(screen.getByRole('combobox'), 'p2')
    await userEvent.click(screen.getByRole('button', { name: /^add$/i }))
    await waitFor(() => expect(screen.getByText(/already in the group's ancestry/i)).toBeInTheDocument())
  })

  it('renders role select and posts with role for project kind', async () => {
    mockApi.addProjectMember = vi.fn().mockResolvedValue(undefined)
    const onAdded = vi.fn()
    render(<AddMemberDialog
      open={true}
      onOpenChange={vi.fn()}
      onAdded={onAdded}
      groupId="proj1"
      parties={parties}
      excludedIds={new Set()}
      existingMemberIds={new Set()}
      kind="project"
      roleOptions={['project:owner', 'project:developer', 'project:viewer']}
      defaultRole="project:viewer"
    />)
    await userEvent.selectOptions(screen.getByLabelText('Member'), 'p2')
    await userEvent.selectOptions(screen.getByLabelText('Role'), 'project:developer')
    await userEvent.click(screen.getByRole('button', { name: /^add$/i }))
    await waitFor(() => expect(mockApi.addProjectMember).toHaveBeenCalledWith('proj1', 'p2', 'project:developer'))
    expect(onAdded).toHaveBeenCalled()
  })
})

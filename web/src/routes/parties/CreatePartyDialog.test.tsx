import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import CreatePartyDialog from './CreatePartyDialog'

vi.mock('@/api', () => ({
  createGroup: vi.fn(),
}))

import * as api from '@/api'
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => {
  vi.clearAllMocks()
})

describe('CreatePartyDialog', () => {
  it('calls createGroup with entered name and fires onCreated', async () => {
    mockApi.createGroup.mockResolvedValue({ id: 'g1', kind: 'group', name: 'team-a', is_system: false, created_at: '', updated_at: '' })
    const onCreated = vi.fn()
    const onOpenChange = vi.fn()
    render(<CreatePartyDialog open={true} onOpenChange={onOpenChange} onCreated={onCreated} />)
    await userEvent.type(screen.getByLabelText(/name/i), 'team-a')
    await userEvent.click(screen.getByRole('button', { name: /^create$/i }))
    await waitFor(() => expect(api.createGroup).toHaveBeenCalledWith({ name: 'team-a' }))
    expect(onCreated).toHaveBeenCalled()
  })

  it('shows error message on API failure', async () => {
    mockApi.createGroup.mockRejectedValue(new Error('duplicate name'))
    render(<CreatePartyDialog open={true} onOpenChange={vi.fn()} onCreated={vi.fn()} />)
    await userEvent.type(screen.getByLabelText(/name/i), 'team-a')
    await userEvent.click(screen.getByRole('button', { name: /^create$/i }))
    await waitFor(() => expect(screen.getByText(/duplicate name/i)).toBeInTheDocument())
  })

  it('disables create button when name is empty', () => {
    render(<CreatePartyDialog open={true} onOpenChange={vi.fn()} onCreated={vi.fn()} />)
    expect(screen.getByRole('button', { name: /^create$/i })).toBeDisabled()
  })
})

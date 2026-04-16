import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import MyProjectsTable from './MyProjectsTable'

vi.mock('@/api', () => ({ getMyProjects: vi.fn() }))
import * as api from '@/api'
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => vi.clearAllMocks())

function renderTable() {
  return render(<MemoryRouter><MyProjectsTable /></MemoryRouter>)
}

describe('MyProjectsTable', () => {
  it('shows empty state when the user has no memberships', async () => {
    mockApi.getMyProjects.mockResolvedValue([])
    renderTable()
    await waitFor(() => expect(screen.getByText(/don't belong to any projects yet/i)).toBeInTheDocument())
  })

  it('renders a row per membership', async () => {
    mockApi.getMyProjects.mockResolvedValue([
      { project: { id: 'p1', kind: 'project', name: 'orion', is_system: false, created_at: '', updated_at: '' }, role: 'project:developer' },
      { project: { id: 'p2', kind: 'project', name: 'nova', is_system: false, created_at: '', updated_at: '' }, role: 'project:viewer' },
    ])
    renderTable()
    await waitFor(() => expect(screen.getByText('orion')).toBeInTheDocument())
    expect(screen.getByText('nova')).toBeInTheDocument()
    expect(screen.getByText('project:developer')).toBeInTheDocument()
    expect(screen.getByText('project:viewer')).toBeInTheDocument()
  })

  it('row is clickable', async () => {
    mockApi.getMyProjects.mockResolvedValue([
      { project: { id: 'p1', kind: 'project', name: 'orion', is_system: false, created_at: '', updated_at: '' }, role: 'project:developer' },
    ])
    renderTable()
    await waitFor(() => expect(screen.getByText('orion')).toBeInTheDocument())
    const row = screen.getByText('orion').closest('tr')
    expect(row).not.toBeNull()
    await userEvent.click(row!)
  })

  it('shows retry UI on error', async () => {
    mockApi.getMyProjects.mockRejectedValue(new Error('boom'))
    renderTable()
    await waitFor(() => expect(screen.getByText(/failed to load projects/i)).toBeInTheDocument())
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument()
  })
})

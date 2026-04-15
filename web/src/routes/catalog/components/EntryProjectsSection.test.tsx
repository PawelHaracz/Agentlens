import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { EntryProjectsSection } from './EntryProjectsSection'

vi.mock('@/api', () => ({ listEntryProjects: vi.fn() }))
import * as api from '@/api'
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => vi.clearAllMocks())

function renderSection(entryId = 'e1') {
  return render(<MemoryRouter><EntryProjectsSection entryId={entryId} /></MemoryRouter>)
}

describe('EntryProjectsSection', () => {
  it('renders nothing when the entry has no projects', async () => {
    mockApi.listEntryProjects.mockResolvedValue([])
    const { container } = renderSection()
    await waitFor(() => expect(mockApi.listEntryProjects).toHaveBeenCalledWith('e1'))
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing on fetch error', async () => {
    mockApi.listEntryProjects.mockRejectedValue(new Error('boom'))
    const { container } = renderSection()
    await waitFor(() => expect(mockApi.listEntryProjects).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('renders a badge link per project', async () => {
    mockApi.listEntryProjects.mockResolvedValue([
      { id: 'p1', kind: 'project', name: 'orion', is_system: false, created_at: '', updated_at: '' },
      { id: 'p2', kind: 'project', name: 'default', is_system: true, created_at: '', updated_at: '' },
    ])
    renderSection()
    await waitFor(() => expect(screen.getByText('orion')).toBeInTheDocument())
    expect(screen.getByText('default')).toBeInTheDocument()
    const orionLink = screen.getByText('orion').closest('a')
    expect(orionLink).toHaveAttribute('href', '/settings/projects/p1')
  })
})

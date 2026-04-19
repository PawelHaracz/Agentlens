import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import ServiceAccountDetailPage from './ServiceAccountDetailPage'

vi.mock('../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}))
vi.mock('@/api', () => ({
  listServiceAccounts: vi.fn(),
  rotateServiceAccountSecret: vi.fn(),
  deleteServiceAccount: vi.fn(),
}))

import { useAuth } from '../contexts/AuthContext'
import * as api from '@/api'

const mockAuth = useAuth as ReturnType<typeof vi.fn>

function renderPage(id: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/admin/service-accounts/${id}`]}>
        <Routes>
          <Route path="/admin/service-accounts/:id" element={<ServiceAccountDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockAuth.mockReturnValue({
    hasPermission: (p: string) =>
      ['service_accounts:read', 'service_accounts:write', 'service_accounts:revoke'].includes(p),
  })
  vi.mocked(api.listServiceAccounts).mockResolvedValue([
    { id: 'sa-1', kind: 'service_account', name: 'my-sa', is_system: false, created_at: '2026-04-01', updated_at: '2026-04-02' },
  ])
})

describe('ServiceAccountDetailPage', () => {
  it('renders account details for a known id', async () => {
    renderPage('sa-1')
    await waitFor(() => expect(screen.getByText('my-sa')).toBeInTheDocument())
    expect(screen.getByTestId('sa-detail-page')).toBeInTheDocument()
  })

  it('shows not-found message for unknown id', async () => {
    renderPage('sa-unknown')
    await waitFor(() => expect(screen.getByText(/Service account not found/i)).toBeInTheDocument())
  })

  it('opens rotate confirm dialog and calls API on confirm', async () => {
    vi.mocked(api.rotateServiceAccountSecret).mockResolvedValue({
      client_id: 'newcid',
      secret: 'agentlens_sk_newcid.rotated',
    })
    renderPage('sa-1')
    await waitFor(() => screen.getByTestId('rotate-secret-btn'))

    await userEvent.click(screen.getByTestId('rotate-secret-btn'))
    expect(screen.getByTestId('rotate-confirm-dialog')).toBeInTheDocument()

    await userEvent.click(screen.getByTestId('rotate-confirm'))
    await waitFor(() => expect(api.rotateServiceAccountSecret).toHaveBeenCalledWith('sa-1'))
  })

  it('opens delete confirm dialog and calls API on confirm', async () => {
    vi.mocked(api.deleteServiceAccount).mockResolvedValue(undefined)
    renderPage('sa-1')
    await waitFor(() => screen.getByTestId('delete-sa-btn'))

    await userEvent.click(screen.getByTestId('delete-sa-btn'))
    expect(screen.getByTestId('delete-confirm-dialog')).toBeInTheDocument()

    await userEvent.click(screen.getByTestId('delete-confirm'))
    await waitFor(() => expect(api.deleteServiceAccount).toHaveBeenCalledWith('sa-1'))
  })
})

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import ServiceAccountsPage from './ServiceAccountsPage'

vi.mock('../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}))
vi.mock('@/api', () => ({
  listServiceAccounts: vi.fn(),
  createServiceAccount: vi.fn(),
  deleteServiceAccount: vi.fn(),
  rotateServiceAccountSecret: vi.fn(),
}))

import { useAuth } from '../contexts/AuthContext'
import * as api from '@/api'

const mockAuth = useAuth as ReturnType<typeof vi.fn>

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <ServiceAccountsPage />
      </MemoryRouter>
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockAuth.mockReturnValue({
    hasPermission: (p: string) => ['service_accounts:read', 'service_accounts:write', 'service_accounts:revoke'].includes(p),
  })
  vi.mocked(api.listServiceAccounts).mockResolvedValue([
    { id: 'sa-1', kind: 'service_account', name: 'my-sa', is_system: false, created_at: '', updated_at: '' },
  ])
})

describe('ServiceAccountsPage', () => {
  it('renders the SA table', async () => {
    renderPage()
    await waitFor(() => expect(screen.getByText('my-sa')).toBeInTheDocument())
    expect(screen.getByTestId('sa-table')).toBeInTheDocument()
  })

  it('opens create modal when button clicked', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('create-sa-btn'))
    await userEvent.click(screen.getByTestId('create-sa-btn'))
    expect(screen.getByTestId('create-sa-dialog')).toBeInTheDocument()
  })

  it('displays one-time secret after create', async () => {
    vi.mocked(api.createServiceAccount).mockResolvedValue({
      party: { id: 'sa-new', kind: 'service_account', name: 'new-sa', is_system: false, created_at: '', updated_at: '' },
      client_id: 'abc123',
      secret: 'agentlens_sk_abc123.supersecret',
      secret_format: 'agentlens_sk_<client_id>.<secret>',
    })
    renderPage()
    await waitFor(() => screen.getByTestId('create-sa-btn'))
    await userEvent.click(screen.getByTestId('create-sa-btn'))
    await userEvent.type(screen.getByTestId('sa-name-input'), 'new-sa')
    await userEvent.click(screen.getByRole('button', { name: /create/i }))
    await waitFor(() => expect(screen.getByTestId('one-time-secret-display')).toBeInTheDocument())
  })
})

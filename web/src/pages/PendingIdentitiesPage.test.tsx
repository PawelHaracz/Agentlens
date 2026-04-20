import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import PendingIdentitiesPage from './PendingIdentitiesPage'

vi.mock('../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}))
vi.mock('@/api', () => ({
  listPendingIdentities: vi.fn(),
  approveIdentity: vi.fn(),
  rejectIdentity: vi.fn(),
}))

import { useAuth } from '../contexts/AuthContext'
import * as api from '@/api'

const mockAuth = useAuth as ReturnType<typeof vi.fn>

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <PendingIdentitiesPage />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockAuth.mockReturnValue({
    hasPermission: (p: string) =>
      ['service_accounts:read', 'service_accounts:write'].includes(p),
  })
  vi.mocked(api.listPendingIdentities).mockResolvedValue([
    { id: 'eid-1', provider_name: 'dex', sub: 'sub-123', email: 'user@dex.test', display_name: '', status: 'pending', created_at: '' },
  ])
  vi.mocked(api.approveIdentity).mockResolvedValue(undefined)
  vi.mocked(api.rejectIdentity).mockResolvedValue(undefined)
})

describe('PendingIdentitiesPage', () => {
  it('renders pending identity table', async () => {
    renderPage()
    await waitFor(() => expect(screen.getByText('sub-123')).toBeInTheDocument())
    expect(screen.getByText('user@dex.test')).toBeInTheDocument()
    expect(screen.getByTestId('pending-identities-table')).toBeInTheDocument()
  })

  it('calls approveIdentity on approve click', async () => {
    const user = userEvent.setup()
    renderPage()
    await waitFor(() => screen.getByTestId('approve-btn-eid-1'))
    await user.click(screen.getByTestId('approve-btn-eid-1'))
    expect(api.approveIdentity).toHaveBeenCalledWith('eid-1')
  })

  it('calls rejectIdentity on reject click', async () => {
    const user = userEvent.setup()
    renderPage()
    await waitFor(() => screen.getByTestId('reject-btn-eid-1'))
    await user.click(screen.getByTestId('reject-btn-eid-1'))
    expect(api.rejectIdentity).toHaveBeenCalledWith('eid-1')
  })

  it('shows access-restricted card when user lacks read permission', async () => {
    mockAuth.mockReturnValue({ hasPermission: () => false })
    renderPage()
    await waitFor(() => expect(screen.getByText(/Access restricted/i)).toBeInTheDocument())
  })

  it('disables approve/reject buttons when user lacks write permission', async () => {
    mockAuth.mockReturnValue({
      hasPermission: (p: string) => p === 'service_accounts:read',
    })
    renderPage()
    await waitFor(() => screen.getByTestId('approve-btn-eid-1'))
    expect(screen.getByTestId('approve-btn-eid-1')).toBeDisabled()
    expect(screen.getByTestId('reject-btn-eid-1')).toBeDisabled()
  })
})

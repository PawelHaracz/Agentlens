import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import LoginPage from './LoginPage'

vi.mock('../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}))

import { useAuth } from '../contexts/AuthContext'

const mockUseAuth = useAuth as ReturnType<typeof vi.fn>

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

beforeEach(() => {
  vi.clearAllMocks()
})

function renderLoginPage() {
  const mockLogin = vi.fn()
  mockUseAuth.mockReturnValue({ login: mockLogin })
  render(
    <MemoryRouter>
      <Routes>
        <Route path="/" element={<LoginPage />} />
        <Route path="/home" element={<div>Home</div>} />
      </Routes>
    </MemoryRouter>
  )
  return { mockLogin }
}

describe('LoginPage', () => {
  it('renders the heading and sign-in button', () => {
    renderLoginPage()
    expect(screen.getByText('AgentLens')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
  })

  it('renders username and password inputs', () => {
    renderLoginPage()
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
  })

  it('calls login with entered credentials on submit', async () => {
    const user = userEvent.setup()
    const { mockLogin } = renderLoginPage()
    mockLogin.mockResolvedValue(undefined)

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'secret')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(mockLogin).toHaveBeenCalledWith('admin', 'secret')
    })
  })

  it('navigates to / after successful login', async () => {
    const user = userEvent.setup()
    const { mockLogin } = renderLoginPage()
    mockLogin.mockResolvedValue(undefined)

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'pass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/')
    })
  })

  it('shows error message on failed login', async () => {
    const user = userEvent.setup()
    const { mockLogin } = renderLoginPage()
    mockLogin.mockRejectedValue(new Error('Invalid credentials'))

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'wrong')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByText('Invalid credentials')).toBeInTheDocument()
    })
  })

  it('disables the submit button while signing in', async () => {
    const user = userEvent.setup()
    const { mockLogin } = renderLoginPage()
    let resolve!: (value: void | PromiseLike<void>) => void
    mockLogin.mockReturnValue(new Promise<void>(r => { resolve = r }))

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'pass')

    const btn = screen.getByRole('button', { name: /sign in/i })
    await user.click(btn)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /signing in/i })).toBeDisabled()
    })

    resolve()
  })
})

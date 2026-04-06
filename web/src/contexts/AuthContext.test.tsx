import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import { AuthProvider, useAuth } from './AuthContext'
import * as api from '../api'

vi.mock('../api', () => ({
  getMe: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(),
  setToken: vi.fn(),
  getToken: vi.fn(),
}))

const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  vi.clearAllMocks()
})

function TestConsumer() {
  const { user, isAuthenticated, isLoading, permissions, hasPermission } = useAuth()
  return (
    <div>
      <span data-testid="loading">{String(isLoading)}</span>
      <span data-testid="authenticated">{String(isAuthenticated)}</span>
      <span data-testid="user">{user?.username ?? 'null'}</span>
      <span data-testid="permissions">{permissions.join(',')}</span>
      <span data-testid="has-catalog">{String(hasPermission('catalog:read'))}</span>
    </div>
  )
}

describe('AuthProvider', () => {
  it('starts with isLoading true and resolves when getMe completes', async () => {
    mockApi.getMe.mockResolvedValue({
      id: '1',
      username: 'alice',
      role: { id: 'r1', name: 'viewer', description: '', is_system: false, permissions: ['catalog:read'] },
    })

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false')
    })
    expect(screen.getByTestId('authenticated').textContent).toBe('true')
    expect(screen.getByTestId('user').textContent).toBe('alice')
  })

  it('sets isAuthenticated=false when getMe fails', async () => {
    mockApi.getMe.mockRejectedValue(new Error('Unauthorized'))

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false')
    })
    expect(screen.getByTestId('authenticated').textContent).toBe('false')
    expect(screen.getByTestId('user').textContent).toBe('null')
  })

  it('exposes permissions from the user role', async () => {
    mockApi.getMe.mockResolvedValue({
      id: '1',
      username: 'alice',
      role: { id: 'r1', name: 'admin', description: '', is_system: false, permissions: ['catalog:read', 'users:write'] },
    })

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId('permissions').textContent).toBe('catalog:read,users:write')
    })
  })

  it('hasPermission returns true for granted permission', async () => {
    mockApi.getMe.mockResolvedValue({
      id: '1',
      username: 'alice',
      role: { id: 'r1', name: 'viewer', description: '', is_system: false, permissions: ['catalog:read'] },
    })

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId('has-catalog').textContent).toBe('true')
    })
  })

  it('login sets the user and token', async () => {
    mockApi.getMe.mockRejectedValue(new Error('not authed'))

    const mockUser = { id: '1', username: 'bob', role: { id: 'r1', name: 'viewer', description: '', is_system: false, permissions: [] } }
    mockApi.login.mockResolvedValue({ token: 'tok123', user: mockUser })

    function LoginButton() {
      const { login, user } = useAuth()
      return (
        <>
          <button onClick={() => login('bob', 'pass')}>Login</button>
          <span data-testid="username">{user?.username ?? 'none'}</span>
        </>
      )
    }

    render(
      <AuthProvider>
        <LoginButton />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId('username').textContent).toBe('none')
    })

    await act(async () => {
      screen.getByRole('button', { name: 'Login' }).click()
    })

    await waitFor(() => {
      expect(screen.getByTestId('username').textContent).toBe('bob')
    })
    expect(mockApi.setToken).toHaveBeenCalledWith('tok123')
  })

  it('logout clears the user and token', async () => {
    mockApi.getMe.mockResolvedValue({
      id: '1',
      username: 'alice',
      role: { id: 'r1', name: 'viewer', description: '', is_system: false, permissions: [] },
    })
    mockApi.logout.mockResolvedValue(undefined)

    function LogoutButton() {
      const { logout, user } = useAuth()
      return (
        <>
          <button onClick={logout}>Logout</button>
          <span data-testid="username">{user?.username ?? 'none'}</span>
        </>
      )
    }

    render(
      <AuthProvider>
        <LogoutButton />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId('username').textContent).toBe('alice')
    })

    await act(async () => {
      screen.getByRole('button', { name: 'Logout' }).click()
    })

    await waitFor(() => {
      expect(screen.getByTestId('username').textContent).toBe('none')
    })
    expect(mockApi.setToken).toHaveBeenCalledWith(null)
  })
})

describe('useAuth outside provider', () => {
  it('throws when used outside AuthProvider', () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => render(<TestConsumer />)).toThrow('useAuth must be used within AuthProvider')
    consoleError.mockRestore()
  })
})

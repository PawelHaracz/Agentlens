import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import App from './App'

vi.mock('./contexts/AuthContext', () => ({
  AuthProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useAuth: vi.fn().mockReturnValue({
    user: null,
    isAuthenticated: false,
    isLoading: false,
    permissions: [],
    login: vi.fn(),
    logout: vi.fn(),
    hasPermission: vi.fn().mockReturnValue(false),
    refreshUser: vi.fn(),
  }),
}))

vi.mock('./contexts/ThemeContext', () => ({
  ThemeProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useTheme: vi.fn().mockReturnValue({ theme: 'light', setTheme: vi.fn() }),
}))

describe('App', () => {
  it('renders without crashing', async () => {
    render(<App />)
    await waitFor(() => {
      expect(document.body).toBeTruthy()
    })
  })

  it('renders login page at /login route', async () => {
    window.history.pushState({}, '', '/login')
    render(<App />)
    await waitFor(() => {
      expect(screen.getByText('AgentLens')).toBeInTheDocument()
    })
  })

  it('redirects unauthenticated users to /login', async () => {
    window.history.pushState({}, '', '/')
    render(<App />)
    await waitFor(() => {
      expect(screen.getByText('AgentLens')).toBeInTheDocument()
    })
  })
})

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import Layout from './Layout'

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

function makeAuthValue(overrides = {}) {
  return {
    user: { id: '1', username: 'alice', display_name: 'Alice Smith', email: 'alice@example.com', role: { permissions: ['catalog:read', 'settings:read'] } },
    isAuthenticated: true,
    isLoading: false,
    permissions: ['catalog:read', 'settings:read'],
    login: vi.fn(),
    logout: vi.fn(),
    hasPermission: (p: string) => ['catalog:read', 'settings:read'].includes(p),
    refreshUser: vi.fn(),
    ...overrides,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
})

function renderLayout(authOverrides = {}) {
  mockUseAuth.mockReturnValue(makeAuthValue(authOverrides))
  return render(
    <MemoryRouter>
      <Layout>
        <div>Page Content</div>
      </Layout>
    </MemoryRouter>
  )
}

describe('Layout', () => {
  it('renders the AgentLens brand link', () => {
    renderLayout()
    expect(screen.getByText('AgentLens')).toBeInTheDocument()
  })

  it('renders children content', () => {
    renderLayout()
    expect(screen.getByText('Page Content')).toBeInTheDocument()
  })

  it('renders Catalog nav link', () => {
    renderLayout()
    expect(screen.getByRole('link', { name: 'Catalog' })).toBeInTheDocument()
  })

  it('renders Settings nav link when user has settings:read permission', () => {
    renderLayout()
    expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument()
  })

  it('does not render Settings nav link without settings:read permission', () => {
    mockUseAuth.mockReturnValue(makeAuthValue({
      hasPermission: (_p: string) => false,
    }))
    render(
      <MemoryRouter>
        <Layout><div /></Layout>
      </MemoryRouter>
    )
    expect(screen.queryByRole('link', { name: 'Settings' })).not.toBeInTheDocument()
  })

  it('renders user initials in the user button', () => {
    renderLayout()
    expect(screen.getByText('AS')).toBeInTheDocument()
  })

  it('shows user dropdown with logout option when button is clicked', async () => {
    const user = userEvent.setup()
    renderLayout()
    await user.click(screen.getByText('AS'))
    await waitFor(() => {
      expect(screen.getByRole('menuitem', { name: /logout/i })).toBeInTheDocument()
    })
  })

  it('calls logout and navigates to /login on logout click', async () => {
    const mockLogout = vi.fn().mockResolvedValue(undefined)
    mockUseAuth.mockReturnValue(makeAuthValue({ logout: mockLogout }))
    const user = userEvent.setup()
    render(
      <MemoryRouter>
        <Layout><div /></Layout>
      </MemoryRouter>
    )
    await user.click(screen.getByText('AS'))
    await waitFor(() => {
      expect(screen.getByRole('menuitem', { name: /logout/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('menuitem', { name: /logout/i }))
    await waitFor(() => {
      expect(mockLogout).toHaveBeenCalled()
      expect(mockNavigate).toHaveBeenCalledWith('/login')
    })
  })

  it('uses username as fallback when display_name is absent', () => {
    mockUseAuth.mockReturnValue(makeAuthValue({
      user: { id: '1', username: 'bob', display_name: '', email: '' },
    }))
    render(
      <MemoryRouter>
        <Layout><div /></Layout>
      </MemoryRouter>
    )
    expect(screen.getByText('B')).toBeInTheDocument()
  })
})

describe('Layout — mobile menu', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('opens mobile nav when hamburger is clicked', async () => {
    const user = userEvent.setup()
    mockUseAuth.mockReturnValue(makeAuthValue())
    render(
      <MemoryRouter>
        <Layout><div /></Layout>
      </MemoryRouter>
    )

    const hamburger = screen.getByRole('button', { name: /Open menu/ })
    await user.click(hamburger)

    const navLinks = screen.getAllByRole('link', { name: 'Catalog' })
    expect(navLinks.length).toBeGreaterThan(1)
  })

  it('closes mobile nav when a link is clicked', async () => {
    const user = userEvent.setup()
    mockUseAuth.mockReturnValue(makeAuthValue())
    render(
      <MemoryRouter>
        <Layout><div /></Layout>
      </MemoryRouter>
    )

    const hamburger = screen.getByRole('button', { name: /Open menu/ })
    await user.click(hamburger)

    const mobileLinks = screen.getAllByRole('link', { name: 'Catalog' })
    await user.click(mobileLinks[mobileLinks.length - 1])

    await waitFor(() => {
      const links = screen.getAllByRole('link', { name: 'Catalog' })
      expect(links.length).toBe(1)
    })
  })
})

describe('Layout — Settings navigation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('navigates to settings from the user dropdown Settings item', async () => {
    const user = userEvent.setup()
    mockUseAuth.mockReturnValue(makeAuthValue())
    render(
      <MemoryRouter>
        <Layout><div /></Layout>
      </MemoryRouter>
    )
    await user.click(screen.getByText('AS'))
    await waitFor(() => {
      expect(screen.getByRole('menuitem', { name: /settings/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('menuitem', { name: /settings/i }))
    expect(mockNavigate).toHaveBeenCalledWith('/settings')
  })

  it('navigates to account settings from the My Account dropdown item', async () => {
    const user = userEvent.setup()
    mockUseAuth.mockReturnValue(makeAuthValue())
    render(
      <MemoryRouter>
        <Layout><div /></Layout>
      </MemoryRouter>
    )
    await user.click(screen.getByText('AS'))
    await waitFor(() => {
      expect(screen.getByRole('menuitem', { name: /my account/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('menuitem', { name: /my account/i }))
    expect(mockNavigate).toHaveBeenCalledWith('/settings?tab=account')
  })
})

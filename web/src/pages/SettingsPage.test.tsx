import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import SettingsPage from './SettingsPage'

vi.mock('../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}))

vi.mock('../contexts/ThemeContext', () => ({
  useTheme: vi.fn(),
}))

vi.mock('../api', () => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  listUsers: vi.fn(),
  listRoles: vi.fn(),
  createUser: vi.fn(),
  updateUser: vi.fn(),
  deleteUser: vi.fn(),
  createRole: vi.fn(),
  updateRole: vi.fn(),
  deleteRole: vi.fn(),
  changePassword: vi.fn(),
}))

import { useAuth } from '../contexts/AuthContext'
import { useTheme } from '../contexts/ThemeContext'
import * as api from '../api'

const mockUseAuth = useAuth as ReturnType<typeof vi.fn>
const mockUseTheme = useTheme as ReturnType<typeof vi.fn>
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

const mockUser = {
  id: 'u1',
  username: 'admin',
  display_name: 'Admin User',
  email: 'admin@example.com',
  role_id: 'r1',
  role: { permissions: [] },
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function makeAuthValue(overrides = {}) {
  return {
    user: mockUser,
    isAuthenticated: true,
    isLoading: false,
    permissions: ['settings:read', 'users:read', 'roles:read', 'users:write', 'users:delete'],
    login: vi.fn(),
    logout: vi.fn(),
    hasPermission: (p: string) => ['settings:read', 'users:read', 'roles:read', 'users:write', 'users:delete'].includes(p),
    refreshUser: vi.fn(),
    ...overrides,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  mockUseAuth.mockReturnValue(makeAuthValue())
  mockUseTheme.mockReturnValue({ theme: 'light', setTheme: vi.fn() })
  mockApi.getSettings.mockResolvedValue([])
  mockApi.listUsers.mockResolvedValue([])
  mockApi.listRoles.mockResolvedValue([])
})

afterEach(() => {
  vi.useRealTimers()
})

function renderSettingsPage() {
  return render(
    <MemoryRouter>
      <SettingsPage />
    </MemoryRouter>
  )
}

describe('SettingsPage', () => {
  it('renders the Settings heading', () => {
    renderSettingsPage()
    expect(screen.getByText('Settings')).toBeInTheDocument()
  })

  it('renders General tab by default', () => {
    renderSettingsPage()
    expect(screen.getByRole('tab', { name: /general/i })).toBeInTheDocument()
    expect(screen.getByText('Appearance')).toBeInTheDocument()
  })

  it('renders Users tab when user has users:read permission', () => {
    renderSettingsPage()
    expect(screen.getByRole('tab', { name: /users/i })).toBeInTheDocument()
  })

  it('hides Users tab without users:read permission', () => {
    mockUseAuth.mockReturnValue(makeAuthValue({
      hasPermission: (_p: string) => false,
    }))
    renderSettingsPage()
    expect(screen.queryByRole('tab', { name: /users/i })).not.toBeInTheDocument()
  })

  it('renders Roles tab when user has roles:read permission', () => {
    renderSettingsPage()
    expect(screen.getByRole('tab', { name: /roles/i })).toBeInTheDocument()
  })

  it('renders My Account tab', () => {
    renderSettingsPage()
    expect(screen.getByRole('tab', { name: /my account/i })).toBeInTheDocument()
  })

  it('shows theme buttons in General tab', () => {
    renderSettingsPage()
    expect(screen.getByRole('button', { name: /light/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /dark/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /system/i })).toBeInTheDocument()
  })

  it('switches to Users tab on click and shows users table', async () => {
    const user = userEvent.setup()
    mockApi.listUsers.mockResolvedValue([
      { ...mockUser, id: 'u2', username: 'bob', email: 'bob@test.com', is_active: true },
    ])
    renderSettingsPage()

    await user.click(screen.getByRole('tab', { name: /users/i }))

    await waitFor(() => {
      expect(screen.getByText('bob')).toBeInTheDocument()
    })
  })

  it('shows "No users found." in Users tab when list is empty', async () => {
    const user = userEvent.setup()
    mockApi.listUsers.mockResolvedValue([])
    renderSettingsPage()

    await user.click(screen.getByRole('tab', { name: /users/i }))

    await waitFor(() => {
      expect(screen.getByText(/no users found/i)).toBeInTheDocument()
    })
  })

  it('switches to My Account tab and shows Profile section', async () => {
    const user = userEvent.setup()
    renderSettingsPage()

    await user.click(screen.getByRole('tab', { name: /my account/i }))

    await waitFor(() => {
      expect(screen.getByText('Profile')).toBeInTheDocument()
    })
  })

  it('shows username as disabled input in Account tab', async () => {
    const user = userEvent.setup()
    renderSettingsPage()

    await user.click(screen.getByRole('tab', { name: /my account/i }))

    await waitFor(() => {
      const usernameInputs = screen.getAllByDisplayValue('admin')
      expect(usernameInputs.some(i => (i as HTMLInputElement).disabled)).toBe(true)
    })
  })

  it('shows password inputs in Account tab', async () => {
    const user = userEvent.setup()
    const { container } = renderSettingsPage()

    await user.click(screen.getByRole('tab', { name: /my account/i }))

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /change password/i })).toBeInTheDocument()
    })

    const passwordInputs = container.querySelectorAll('input[type="password"]')
    expect(passwordInputs).toHaveLength(3)
  })

  it('shows the Change password submit button in Account tab', async () => {
    const user = userEvent.setup()
    renderSettingsPage()

    await user.click(screen.getByRole('tab', { name: /my account/i }))

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /change password/i })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /change password/i })).toBeInTheDocument()
    })
  })

  it('opens Add User dialog from Users tab', async () => {
    const user = userEvent.setup()
    renderSettingsPage()

    await user.click(screen.getByRole('tab', { name: /users/i }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument()
    })

    const addUserButtons = screen.getAllByRole('button', { name: /add user/i })
    await user.click(addUserButtons[0])

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })
  })

  it('saves settings via Save settings button in General tab', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

    mockApi.updateSettings.mockResolvedValue(undefined)
    renderSettingsPage()

    await user.click(screen.getByRole('button', { name: /save settings/i }))

    await waitFor(() => {
      expect(mockApi.updateSettings).toHaveBeenCalled()
    })

    vi.runOnlyPendingTimers()
  })
})

describe('SettingsPage — General tab interactions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuth.mockReturnValue(makeAuthValue())
    mockUseTheme.mockReturnValue({ theme: 'light', setTheme: vi.fn() })
    mockApi.getSettings.mockResolvedValue([])
    mockApi.listUsers.mockResolvedValue([])
    mockApi.listRoles.mockResolvedValue([])
  })

  it('calls setTheme when Dark button is clicked', async () => {
    const mockSetTheme = vi.fn()
    mockUseTheme.mockReturnValue({ theme: 'light', setTheme: mockSetTheme })
    mockApi.updateSettings.mockResolvedValue(undefined)

    const user = userEvent.setup()
    render(<MemoryRouter><SettingsPage /></MemoryRouter>)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /dark/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /dark/i }))
    expect(mockSetTheme).toHaveBeenCalledWith('dark')
  })

  it('shows "Settings saved." message after successful save', async () => {
    const user = userEvent.setup()
    mockApi.updateSettings.mockResolvedValue(undefined)
    render(<MemoryRouter><SettingsPage /></MemoryRouter>)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /save settings/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /save settings/i }))
    await waitFor(() => {
      expect(screen.getByText('Settings saved.')).toBeInTheDocument()
    })
  })

  it('applies theme from settings response', async () => {
    const mockSetTheme = vi.fn()
    mockUseTheme.mockReturnValue({ theme: 'light', setTheme: mockSetTheme })
    mockApi.getSettings.mockResolvedValue([
      { key: 'ui.theme', value: 'dark', category: 'ui', description: '' },
    ])
    render(<MemoryRouter><SettingsPage /></MemoryRouter>)

    await waitFor(() => {
      expect(mockSetTheme).toHaveBeenCalledWith('dark')
    })
  })
})

describe('SettingsPage — Users tab CRUD', () => {
  const mockUsers = [
    {
      id: 'u2',
      username: 'bob',
      email: 'bob@test.com',
      display_name: 'Bob',
      role_id: 'r1',
      is_active: true,
      created_at: '',
      updated_at: '',
    },
  ]

  const mockRoles = [
    { id: 'r1', name: 'admin', description: '', permissions: [], is_system: true },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuth.mockReturnValue(makeAuthValue())
    mockUseTheme.mockReturnValue({ theme: 'light', setTheme: vi.fn() })
    mockApi.getSettings.mockResolvedValue([])
    mockApi.listUsers.mockResolvedValue(mockUsers)
    mockApi.listRoles.mockResolvedValue(mockRoles)
    mockApi.updateSettings.mockResolvedValue(undefined)
  })

  async function openUsersTab() {
    const user = userEvent.setup()
    render(<MemoryRouter><SettingsPage /></MemoryRouter>)
    await user.click(screen.getByRole('tab', { name: /users/i }))
    await waitFor(() => {
      expect(screen.getByText('bob')).toBeInTheDocument()
    })
    return user
  }

  it('renders existing users in the Users tab', async () => {
    await openUsersTab()
    expect(screen.getByText('bob')).toBeInTheDocument()
    expect(screen.getByText('bob@test.com')).toBeInTheDocument()
  })

  it('shows Active badge for active users', async () => {
    await openUsersTab()
    expect(screen.getByText('Active')).toBeInTheDocument()
  })

  it('creates a new user via the Add User dialog', async () => {
    mockApi.createUser.mockResolvedValue({ id: 'u3', username: 'charlie' })
    const { fireEvent } = await import('@testing-library/react')
    const user = await openUsersTab()

    const addButtons = screen.getAllByRole('button', { name: /add user/i })
    await user.click(addButtons[0])

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })

    const inputs = screen.getByRole('dialog').querySelectorAll('input')
    fireEvent.change(inputs[0], { target: { value: 'charlie' } })
    fireEvent.change(inputs[3], { target: { value: 'secret1234' } })

    const createBtn = screen.getByRole('button', { name: /create/i })
    await user.click(createBtn)

    await waitFor(() => {
      expect(mockApi.createUser).toHaveBeenCalled()
    })
  })

  it('opens edit dialog when clicking Edit on an existing user', async () => {
    const user = await openUsersTab()

    const editBtn = screen.getByTitle('Edit')
    await user.click(editBtn)

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
      expect(screen.getByText('Edit user')).toBeInTheDocument()
    })
  })

  it('calls deleteUser when confirming delete', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    try {
      mockApi.deleteUser.mockResolvedValue(undefined)
      const user = await openUsersTab()

      const deleteBtn = screen.getByTitle('Delete')
      await user.click(deleteBtn)

      await waitFor(() => {
        expect(mockApi.deleteUser).toHaveBeenCalledWith('u2')
      })
    } finally {
      confirmSpy.mockRestore()
    }
  })

  it('calls updateUser when toggling lock', async () => {
    mockApi.updateUser.mockResolvedValue(undefined)
    const user = await openUsersTab()

    const lockBtn = screen.getByTitle('Lock')
    await user.click(lockBtn)

    await waitFor(() => {
      expect(mockApi.updateUser).toHaveBeenCalledWith('u2', { is_active: false })
    })
  })
})

describe('SettingsPage — Roles tab', () => {
  const mockRoles = [
    { id: 'r1', name: 'admin', description: 'Administrator', permissions: ['catalog:read'], is_system: false },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuth.mockReturnValue(makeAuthValue({
      hasPermission: (p: string) => ['roles:read', 'roles:write', 'users:read'].includes(p),
    }))
    mockUseTheme.mockReturnValue({ theme: 'light', setTheme: vi.fn() })
    mockApi.getSettings.mockResolvedValue([])
    mockApi.listUsers.mockResolvedValue([])
    mockApi.listRoles.mockResolvedValue(mockRoles)
  })

  async function openRolesTab() {
    const user = userEvent.setup()
    render(<MemoryRouter><SettingsPage /></MemoryRouter>)
    await user.click(screen.getByRole('tab', { name: /roles/i }))
    await waitFor(() => {
      expect(screen.getByText('admin')).toBeInTheDocument()
    })
    return user
  }

  it('renders existing roles in the Roles tab', async () => {
    await openRolesTab()
    expect(screen.getByText('admin')).toBeInTheDocument()
    expect(screen.getByText('catalog:read')).toBeInTheDocument()
  })

  it('shows "No roles found." when roles list is empty', async () => {
    mockApi.listRoles.mockResolvedValue([])
    const user = userEvent.setup()
    render(<MemoryRouter><SettingsPage /></MemoryRouter>)
    await user.click(screen.getByRole('tab', { name: /roles/i }))
    await waitFor(() => {
      expect(screen.getByText(/no roles found/i)).toBeInTheDocument()
    })
  })

  it('opens Add role dialog', async () => {
    const user = await openRolesTab()
    const addRoleButtons = screen.getAllByRole('button', { name: /add role/i })
    await user.click(addRoleButtons[0])
    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })
  })

  it('opens Edit role dialog when clicking Edit on a non-system role', async () => {
    const user = await openRolesTab()
    const editBtn = screen.getByTitle('Edit')
    await user.click(editBtn)
    await waitFor(() => {
      expect(screen.getByText('Edit role')).toBeInTheDocument()
    })
  })

  it('creates a role via Add role dialog', async () => {
    mockApi.createRole.mockResolvedValue({ id: 'r2', name: 'editor', permissions: [] })
    const { fireEvent } = await import('@testing-library/react')
    const user = await openRolesTab()
    const addRoleButtons = screen.getAllByRole('button', { name: /add role/i })
    await user.click(addRoleButtons[0])

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })

    const nameInput = screen.getByRole('dialog').querySelector('input')!
    fireEvent.change(nameInput, { target: { value: 'editor' } })

    const createBtn = screen.getByRole('button', { name: /create/i })
    await user.click(createBtn)

    await waitFor(() => {
      expect(mockApi.createRole).toHaveBeenCalled()
    })
  })

  it('deletes a role when confirmed', async () => {
    window.confirm = vi.fn().mockReturnValue(true)
    mockApi.deleteRole.mockResolvedValue(undefined)
    const user = await openRolesTab()

    const deleteBtn = screen.getByTitle('Delete')
    await user.click(deleteBtn)

    await waitFor(() => {
      expect(mockApi.deleteRole).toHaveBeenCalledWith('r1')
    })
  })

  it('toggles permissions in the role dialog', async () => {
    const user = await openRolesTab()
    const addRoleButtons = screen.getAllByRole('button', { name: /add role/i })
    await user.click(addRoleButtons[0])

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })

    const checkbox = screen.getAllByRole('checkbox')[0]
    await user.click(checkbox)
    expect(checkbox).toBeChecked()
  })
})

describe('SettingsPage — Account tab profile update', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuth.mockReturnValue(makeAuthValue())
    mockUseTheme.mockReturnValue({ theme: 'light', setTheme: vi.fn() })
    mockApi.getSettings.mockResolvedValue([])
    mockApi.listUsers.mockResolvedValue([])
    mockApi.listRoles.mockResolvedValue([])
  })

  it('submits profile update form', async () => {
    mockApi.updateUser.mockResolvedValue(mockUser)
    const refreshUser = vi.fn().mockResolvedValue(undefined)
    mockUseAuth.mockReturnValue(makeAuthValue({ refreshUser }))

    const user = userEvent.setup()
    render(<MemoryRouter><SettingsPage /></MemoryRouter>)
    await user.click(screen.getByRole('tab', { name: /my account/i }))

    await waitFor(() => {
      expect(screen.getByText('Profile')).toBeInTheDocument()
    })

    const updateBtn = screen.getByRole('button', { name: /update profile/i })
    await user.click(updateBtn)

    await waitFor(() => {
      expect(mockApi.updateUser).toHaveBeenCalled()
    })
  })
})

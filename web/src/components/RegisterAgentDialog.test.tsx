import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import RegisterAgentDialog from './RegisterAgentDialog'

vi.mock('../api', () => ({
  validateAgentCard: vi.fn(),
  createAgentFromCard: vi.fn(),
  importCardFromURL: vi.fn(),
}))

describe('RegisterAgentDialog', () => {
  const onRegistered = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the Register Agent trigger button', () => {
    render(<RegisterAgentDialog onRegistered={onRegistered} />)
    expect(screen.getByRole('button', { name: /register agent/i })).toBeInTheDocument()
  })

  it('opens dialog when Register Agent button is clicked', async () => {
    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)

    await user.click(screen.getByRole('button', { name: /register agent/i }))

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText('Paste JSON')).toBeInTheDocument()
    expect(screen.getByText('Upload File')).toBeInTheDocument()
  })

  it('shows textarea in the Paste JSON tab', async () => {
    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)

    await user.click(screen.getByRole('button', { name: /register agent/i }))

    const textarea = screen.getByRole('textbox')
    expect(textarea).toBeInTheDocument()
    expect(textarea.tagName).toBe('TEXTAREA')
  })

  it('shows "Invalid JSON syntax" error when JSON is invalid', async () => {
    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)

    await user.click(screen.getByRole('button', { name: /register agent/i }))

    const textarea = screen.getByRole('textbox')
    await user.type(textarea, 'not valid json')

    expect(screen.getByText('Invalid JSON syntax')).toBeInTheDocument()
  })

  it('Validate button is disabled when textarea is empty', async () => {
    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)

    await user.click(screen.getByRole('button', { name: /register agent/i }))

    const validateBtn = screen.getByRole('button', { name: /validate/i })
    expect(validateBtn).toBeDisabled()
  })

  it('Validate button is disabled when JSON is invalid', async () => {
    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)

    await user.click(screen.getByRole('button', { name: /register agent/i }))

    const textarea = screen.getByRole('textbox')
    await user.clear(textarea)
    // Use fireEvent to avoid userEvent's special char parsing for braces
    const { fireEvent } = await import('@testing-library/react')
    fireEvent.change(textarea, { target: { value: '{invalid' } })

    const validateBtn = screen.getByRole('button', { name: /validate/i })
    expect(validateBtn).toBeDisabled()
  })

  it('Validate button is enabled when valid JSON is entered', async () => {
    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)

    await user.click(screen.getByRole('button', { name: /register agent/i }))

    const textarea = screen.getByRole('textbox')
    // Use fireEvent to avoid userEvent's special char parsing for braces
    const { fireEvent } = await import('@testing-library/react')
    fireEvent.change(textarea, { target: { value: '{"name":"test"}' } })

    await waitFor(() => {
      const validateBtn = screen.getByRole('button', { name: /validate/i })
      expect(validateBtn).toBeEnabled()
    })
  })
})

import { importCardFromURL } from '../api'

const mockImportCardFromURL = importCardFromURL as ReturnType<typeof vi.fn>

describe('RegisterAgentDialog — Import from URL tab', () => {
  const onRegistered = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  async function openImportTab() {
    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)
    await user.click(screen.getByRole('button', { name: /register agent/i }))
    await user.click(screen.getByRole('tab', { name: /import from url/i }))
    return user
  }

  it('renders the URL input in Import from URL tab', async () => {
    await openImportTab()
    expect(screen.getByPlaceholderText(/https:\/\/example.com/)).toBeInTheDocument()
  })

  it('Fetch & Import button is disabled when URL is empty', async () => {
    await openImportTab()
    expect(screen.getByRole('button', { name: /fetch.*import/i })).toBeDisabled()
  })

  it('Fetch & Import button is enabled when URL is provided', async () => {
    const user = await openImportTab()
    await user.type(screen.getByPlaceholderText(/https:\/\/example.com/), 'https://example.com/agent.json')
    expect(screen.getByRole('button', { name: /fetch.*import/i })).toBeEnabled()
  })

  it('calls importCardFromURL with the entered URL on import', async () => {
    mockImportCardFromURL.mockResolvedValue({})
    const user = await openImportTab()
    await user.type(screen.getByPlaceholderText(/https:\/\/example.com/), 'https://example.com/agent.json')
    await user.click(screen.getByRole('button', { name: /fetch.*import/i }))
    await waitFor(() => {
      expect(mockImportCardFromURL).toHaveBeenCalledWith(
        expect.objectContaining({ url: 'https://example.com/agent.json' }),
      )
    })
  })

  it('calls onRegistered after successful import', async () => {
    mockImportCardFromURL.mockResolvedValue({})
    const user = await openImportTab()
    await user.type(screen.getByPlaceholderText(/https:\/\/example.com/), 'https://example.com/agent.json')
    await user.click(screen.getByRole('button', { name: /fetch.*import/i }))
    await waitFor(() => {
      expect(onRegistered).toHaveBeenCalled()
    })
  })

  it('shows error message when import fails', async () => {
    mockImportCardFromURL.mockRejectedValue(new Error('could not fetch the resource'))
    const user = await openImportTab()
    await user.type(screen.getByPlaceholderText(/https:\/\/example.com/), 'https://bad-url.example.com')
    await user.click(screen.getByRole('button', { name: /fetch.*import/i }))
    await waitFor(() => {
      expect(screen.getByText(/could not reach the url/i)).toBeInTheDocument()
    })
  })

  it('shows "already exists" error for duplicate endpoint', async () => {
    mockImportCardFromURL.mockRejectedValue(new Error('endpoint already exists'))
    const user = await openImportTab()
    await user.type(screen.getByPlaceholderText(/https:\/\/example.com/), 'https://dupe.example.com')
    await user.click(screen.getByRole('button', { name: /fetch.*import/i }))
    await waitFor(() => {
      expect(screen.getByText(/already exists/i)).toBeInTheDocument()
    })
  })

  it('shows invalid card error for bad agent card URL', async () => {
    mockImportCardFromURL.mockRejectedValue(new Error('not a valid agent card'))
    const user = await openImportTab()
    await user.type(screen.getByPlaceholderText(/https:\/\/example.com/), 'https://notacard.example.com')
    await user.click(screen.getByRole('button', { name: /fetch.*import/i }))
    await waitFor(() => {
      expect(screen.getByText(/valid agent card/i)).toBeInTheDocument()
    })
  })

  it('shows generic error message for unknown errors', async () => {
    mockImportCardFromURL.mockRejectedValue(new Error('timeout'))
    const user = await openImportTab()
    await user.type(screen.getByPlaceholderText(/https:\/\/example.com/), 'https://timeout.example.com')
    await user.click(screen.getByRole('button', { name: /fetch.*import/i }))
    await waitFor(() => {
      expect(screen.getByText('timeout')).toBeInTheDocument()
    })
  })
})

describe('RegisterAgentDialog — Validate flow', () => {
  const onRegistered = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows validation errors when card is invalid', async () => {
    const { validateAgentCard } = await import('../api')
    const mockValidate = validateAgentCard as ReturnType<typeof vi.fn>
    mockValidate.mockResolvedValue({
      valid: false,
      spec_version: '',
      errors: [{ field: 'name', message: 'is required' }],
      warnings: [],
    })

    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)
    await user.click(screen.getByRole('button', { name: /register agent/i }))

    const textarea = screen.getByRole('textbox')
    const { fireEvent } = await import('@testing-library/react')
    fireEvent.change(textarea, { target: { value: '{"version":"1.0"}' } })

    await user.click(screen.getByRole('button', { name: /validate/i }))

    await waitFor(() => {
      expect(screen.getByText(/validation errors/i)).toBeInTheDocument()
    })
  })

  it('shows preview on valid card and allows registration', async () => {
    const { validateAgentCard, createAgentFromCard } = await import('../api')
    const mockValidate = validateAgentCard as ReturnType<typeof vi.fn>
    const mockCreate = createAgentFromCard as ReturnType<typeof vi.fn>
    mockValidate.mockResolvedValue({
      valid: true,
      spec_version: '1.0',
      errors: [],
      warnings: [],
      preview: { display_name: 'Bot', description: 'A bot', protocol: 'a2a' },
    })
    mockCreate.mockResolvedValue({})

    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)
    await user.click(screen.getByRole('button', { name: /register agent/i }))

    const textarea = screen.getByRole('textbox')
    const { fireEvent } = await import('@testing-library/react')
    fireEvent.change(textarea, { target: { value: '{"name":"Bot"}' } })

    await user.click(screen.getByRole('button', { name: /validate/i }))

    await waitFor(() => {
      expect(screen.getByText(/card validated successfully/i)).toBeInTheDocument()
      expect(screen.getByText('Bot')).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /register agent/i }))

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled()
      expect(onRegistered).toHaveBeenCalled()
    })
  })

  it('shows register error when registration fails', async () => {
    const { validateAgentCard, createAgentFromCard } = await import('../api')
    const mockValidate = validateAgentCard as ReturnType<typeof vi.fn>
    const mockCreate = createAgentFromCard as ReturnType<typeof vi.fn>
    mockValidate.mockResolvedValue({
      valid: true,
      spec_version: '1.0',
      errors: [],
      warnings: [],
      preview: { display_name: 'Bot', description: '', protocol: 'a2a' },
    })
    mockCreate.mockRejectedValue(new Error('Registration failed'))

    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)
    await user.click(screen.getByRole('button', { name: /register agent/i }))

    const textarea = screen.getByRole('textbox')
    const { fireEvent } = await import('@testing-library/react')
    fireEvent.change(textarea, { target: { value: '{"name":"Bot"}' } })

    await user.click(screen.getByRole('button', { name: /validate/i }))

    await waitFor(() => {
      expect(screen.getByText(/card validated successfully/i)).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /register agent/i }))

    await waitFor(() => {
      expect(screen.getByText('Registration failed')).toBeInTheDocument()
    })
  })

  it('shows validation warnings in validation step', async () => {
    const { validateAgentCard } = await import('../api')
    const mockValidate = validateAgentCard as ReturnType<typeof vi.fn>
    mockValidate.mockResolvedValue({
      valid: true,
      spec_version: '1.0',
      errors: [],
      warnings: ['Missing optional field: description'],
    })

    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)
    await user.click(screen.getByRole('button', { name: /register agent/i }))

    const textarea = screen.getByRole('textbox')
    const { fireEvent } = await import('@testing-library/react')
    fireEvent.change(textarea, { target: { value: '{"name":"Bot"}' } })

    await user.click(screen.getByRole('button', { name: /validate/i }))

    await waitFor(() => {
      expect(screen.getByText(/missing optional field/i)).toBeInTheDocument()
    })
  })

  it('shows network error when validate throws', async () => {
    const { validateAgentCard } = await import('../api')
    const mockValidate = validateAgentCard as ReturnType<typeof vi.fn>
    mockValidate.mockRejectedValue(new Error('Network error'))

    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)
    await user.click(screen.getByRole('button', { name: /register agent/i }))

    const textarea = screen.getByRole('textbox')
    const { fireEvent } = await import('@testing-library/react')
    fireEvent.change(textarea, { target: { value: '{"name":"Bot"}' } })

    await user.click(screen.getByRole('button', { name: /validate/i }))

    await waitFor(() => {
      expect(screen.getByText(/validation errors/i)).toBeInTheDocument()
    })
  })
})

describe('RegisterAgentDialog — URL import with explicit protocol', () => {
  const onRegistered = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('omits protocol when set to auto (default)', async () => {
    const { importCardFromURL } = await import('../api')
    const mockImport = importCardFromURL as ReturnType<typeof vi.fn>
    mockImport.mockResolvedValue({})

    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)
    await user.click(screen.getByRole('button', { name: /register agent/i }))
    await user.click(screen.getByRole('tab', { name: /import from url/i }))

    await user.type(
      screen.getByPlaceholderText(/https:\/\/example.com/),
      'https://example.com/agent.json'
    )

    await user.click(screen.getByRole('button', { name: /fetch.*import/i }))
    await waitFor(() => {
      expect(mockImport).toHaveBeenCalledWith(
        expect.not.objectContaining({ protocol: expect.anything() })
      )
    })
  })
})

describe('RegisterAgentDialog — Validation back navigation', () => {
  const onRegistered = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('goes back to input step when Back to Edit is clicked in validation step', async () => {
    const { validateAgentCard } = await import('../api')
    const mockValidate = validateAgentCard as ReturnType<typeof vi.fn>
    mockValidate.mockResolvedValue({
      valid: false,
      spec_version: '',
      errors: [{ field: 'name', message: 'required' }],
      warnings: [],
    })

    const user = userEvent.setup()
    render(<RegisterAgentDialog onRegistered={onRegistered} />)
    await user.click(screen.getByRole('button', { name: /register agent/i }))

    const { fireEvent } = await import('@testing-library/react')
    const textarea = screen.getByRole('textbox')
    fireEvent.change(textarea, { target: { value: '{"v":"1"}' } })

    await user.click(screen.getByRole('button', { name: /validate/i }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /back to edit/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /back to edit/i }))

    await waitFor(() => {
      expect(screen.getByRole('textbox')).toBeInTheDocument()
    })
  })
})

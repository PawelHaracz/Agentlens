import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import RegisterAgentDialog from './RegisterAgentDialog'

vi.mock('../api', () => ({
  validateAgentCard: vi.fn(),
  createAgentFromCard: vi.fn(),
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

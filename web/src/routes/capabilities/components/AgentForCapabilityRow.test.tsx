import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import React from 'react'
import { AgentForCapabilityRow } from './AgentForCapabilityRow'
import type { CapabilityAgentDTO } from '@/types'

const AGENT: CapabilityAgentDTO = {
  id: 'agent-1',
  display_name: 'Translator Agent',
  protocol: 'a2a',
  provider: { organization: 'ACME Corp', url: 'https://acme.example.com' },
  health: { state: 'active', latencyMs: 42 },
  spec_version: '1.0',
  status: 'active',
  capability_snippet: { description: 'Translate text between languages' },
}

function renderRow(agent: CapabilityAgentDTO) {
  return render(
    React.createElement(
      MemoryRouter,
      null,
      React.createElement('table', null,
        React.createElement('tbody', null,
          React.createElement(AgentForCapabilityRow, { agent })
        )
      )
    )
  )
}

describe('AgentForCapabilityRow', () => {
  it('renders agent display name', () => {
    renderRow(AGENT)
    expect(screen.getByText('Translator Agent')).toBeInTheDocument()
  })

  it('renders provider organization', () => {
    renderRow(AGENT)
    expect(screen.getByText('ACME Corp')).toBeInTheDocument()
  })

  it('renders spec version', () => {
    renderRow(AGENT)
    expect(screen.getByText('1.0')).toBeInTheDocument()
  })

  it('renders capability snippet description', () => {
    renderRow(AGENT)
    expect(screen.getByText('Translate text between languages')).toBeInTheDocument()
  })

  it('renders latency', () => {
    renderRow(AGENT)
    expect(screen.getByText('42ms')).toBeInTheDocument()
  })

  it('shows dash when provider is null', () => {
    renderRow({ ...AGENT, provider: null })
    expect(screen.getByText('-')).toBeInTheDocument()
  })

  it('truncates long descriptions', () => {
    const longDesc = 'a'.repeat(110)
    renderRow({ ...AGENT, capability_snippet: { description: longDesc } })
    const truncated = 'a'.repeat(100) + '...'
    expect(screen.getByText(truncated)).toBeInTheDocument()
  })

  it('renders dash when spec_version is empty', () => {
    renderRow({ ...AGENT, spec_version: '' })
    const dashes = screen.getAllByText('-')
    expect(dashes.length).toBeGreaterThanOrEqual(1)
  })

  it('renders agent link pointing to /catalog/:id', () => {
    renderRow(AGENT)
    const link = screen.getByRole('link', { name: 'Translator Agent' })
    expect(link).toHaveAttribute('href', '/catalog/agent-1')
  })
})

import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import React from 'react'
import { CapabilityGroup } from './CapabilityGroup'
import type { CapabilityInstance } from '@/types'

const ITEMS: CapabilityInstance[] = [
  {
    kind: 'a2a.skill',
    name: 'translate',
    description: 'Translate text between languages',
    tags: ['nlp', 'translation'],
    input_modes: null,
    output_modes: null,
    agent_id: 'agent-1',
    agent_name: 'Translator Agent',
    protocol: 'a2a',
    status: 'active',
    spec_version: '1.0',
    provider_org: 'ACME',
    provider_url: null,
    health_state: 'active',
    latency_ms: 42,
  },
  {
    kind: 'a2a.skill',
    name: 'translate',
    description: 'Translate text between languages',
    tags: null,
    input_modes: null,
    output_modes: null,
    agent_id: 'agent-2',
    agent_name: 'Second Agent',
    protocol: 'a2a',
    status: 'active',
    spec_version: '1.0',
    provider_org: null,
    provider_url: null,
    health_state: 'active',
    latency_ms: 10,
  },
]

const wrapper = ({ children }: { children: React.ReactNode }) =>
  React.createElement(MemoryRouter, null, children)

describe('CapabilityGroup', () => {
  it('renders capability name', () => {
    render(<CapabilityGroup kind="a2a.skill" name="translate" items={ITEMS} />, { wrapper })
    expect(screen.getByText('translate')).toBeInTheDocument()
  })

  it('renders kind badge', () => {
    render(<CapabilityGroup kind="a2a.skill" name="translate" items={ITEMS} />, { wrapper })
    expect(screen.getByText('A2A Skill')).toBeInTheDocument()
  })

  it('renders agent count', () => {
    render(<CapabilityGroup kind="a2a.skill" name="translate" items={ITEMS} />, { wrapper })
    expect(screen.getByText('2 agents')).toBeInTheDocument()
  })

  it('renders description from first item', () => {
    render(<CapabilityGroup kind="a2a.skill" name="translate" items={ITEMS} />, { wrapper })
    expect(screen.getByText('Translate text between languages')).toBeInTheDocument()
  })

  it('renders tags', () => {
    render(<CapabilityGroup kind="a2a.skill" name="translate" items={ITEMS} />, { wrapper })
    expect(screen.getByText('nlp')).toBeInTheDocument()
    expect(screen.getByText('translation')).toBeInTheDocument()
  })

  it('is collapsed by default', () => {
    render(<CapabilityGroup kind="a2a.skill" name="translate" items={ITEMS} />, { wrapper })
    // Agent names should not be visible when collapsed
    expect(screen.queryByText('Translator Agent')).not.toBeInTheDocument()
  })

  it('expands on click to show agents', () => {
    render(<CapabilityGroup kind="a2a.skill" name="translate" items={ITEMS} />, { wrapper })
    const btn = screen.getByRole('button')
    fireEvent.click(btn)
    expect(screen.getByText('Translator Agent')).toBeInTheDocument()
    expect(screen.getByText('Second Agent')).toBeInTheDocument()
  })

  it('shows View all link when expanded', () => {
    render(<CapabilityGroup kind="a2a.skill" name="translate" items={ITEMS} />, { wrapper })
    fireEvent.click(screen.getByRole('button'))
    expect(screen.getByText(/View all/i)).toBeInTheDocument()
  })

  it('collapses again on second click', () => {
    render(<CapabilityGroup kind="a2a.skill" name="translate" items={ITEMS} />, { wrapper })
    const btn = screen.getByRole('button')
    fireEvent.click(btn)
    fireEvent.click(btn)
    expect(screen.queryByText('Translator Agent')).not.toBeInTheDocument()
  })

  it('renders single agent count correctly', () => {
    render(<CapabilityGroup kind="mcp.tool" name="search" items={[ITEMS[0]]} />, { wrapper })
    expect(screen.getByText('1 agent')).toBeInTheDocument()
  })

  it('shows overflow tag badge when more than 5 tags', () => {
    const manyTags = { ...ITEMS[0], tags: ['a', 'b', 'c', 'd', 'e', 'f', 'g'] }
    render(<CapabilityGroup kind="a2a.skill" name="translate" items={[manyTags]} />, { wrapper })
    expect(screen.getByText('+2 more')).toBeInTheDocument()
  })

  it('renders MCP Tool kind label', () => {
    render(<CapabilityGroup kind="mcp.tool" name="search" items={[{ ...ITEMS[0], kind: 'mcp.tool' }]} />, { wrapper })
    expect(screen.getByText('MCP Tool')).toBeInTheDocument()
  })

  it('renders unknown kind as-is', () => {
    render(<CapabilityGroup kind="custom.kind" name="thing" items={[{ ...ITEMS[0], kind: 'custom.kind' }]} />, { wrapper })
    expect(screen.getByText('custom.kind')).toBeInTheDocument()
  })
})

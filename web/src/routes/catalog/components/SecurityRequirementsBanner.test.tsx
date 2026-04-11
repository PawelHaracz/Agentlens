import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SecurityRequirementsBanner } from './SecurityRequirementsBanner'
import type { SecurityRequirement } from '@/lib/securityUtils'

describe('SecurityRequirementsBanner', () => {
  it('renders null when all requirements are per-skill (no top-level)', () => {
    const requirements: SecurityRequirement[] = [
      { schemes: { httpAuth: [] }, skill_ref: 'someSkill' },
    ]
    const { container } = render(<SecurityRequirementsBanner requirements={requirements} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders null when requirements array is empty', () => {
    const { container } = render(<SecurityRequirementsBanner requirements={[]} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders banner with single top-level requirement', () => {
    const requirements: SecurityRequirement[] = [{ schemes: { httpAuth: [] } }]
    render(<SecurityRequirementsBanner requirements={requirements} />)
    expect(screen.getByText('Required to connect')).toBeDefined()
    expect(screen.getByText('httpAuth')).toBeDefined()
  })

  it('renders "Any of the following" prefix for multiple top-level requirements', () => {
    const requirements: SecurityRequirement[] = [
      { schemes: { httpAuth: [] } },
      { schemes: { apiKeyAuth: [] } },
    ]
    render(<SecurityRequirementsBanner requirements={requirements} />)
    expect(screen.getByText(/Any of the following combinations/)).toBeDefined()
    expect(screen.getByText('httpAuth')).toBeDefined()
    expect(screen.getByText('apiKeyAuth')).toBeDefined()
  })

  it('renders scopes when present', () => {
    const requirements: SecurityRequirement[] = [
      { schemes: { oauth2Auth: ['read:agents', 'write:agents'] } },
    ]
    render(<SecurityRequirementsBanner requirements={requirements} />)
    expect(screen.getByText(/read:agents, write:agents/)).toBeDefined()
  })

  it('renders AND separator for multi-scheme requirement', () => {
    const requirements: SecurityRequirement[] = [
      { schemes: { httpAuth: [], apiKeyAuth: [] } },
    ]
    render(<SecurityRequirementsBanner requirements={requirements} />)
    expect(screen.getByText(/AND/)).toBeDefined()
  })

  it('ignores per-skill requirements and only shows top-level', () => {
    const requirements: SecurityRequirement[] = [
      { schemes: { httpAuth: [] } },
      { schemes: { apiKeyAuth: [] }, skill_ref: 'mySkill' },
    ]
    render(<SecurityRequirementsBanner requirements={requirements} />)
    // Only one list item (the top-level one)
    const items = document.querySelectorAll('li')
    expect(items.length).toBe(1)
    expect(screen.getByText('httpAuth')).toBeDefined()
  })
})

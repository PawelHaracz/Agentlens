import type { CatalogEntry, ListFilter, Stats, ValidationResult, LifecycleState, Health, CapabilityListResult, CapabilityDetailResponse } from './types'

const BASE = '/api/v1'

/* ─── Auth-related types ─── */

export interface User {
  id: string
  username: string
  email: string
  display_name: string
  role_id: string
  role?: Role
  is_active: boolean
  last_login?: string
  created_at: string
  updated_at: string
}

export interface Role {
  id: string
  name: string
  description: string
  permissions: string[]
  is_system: boolean
}

export interface Setting {
  key: string
  value: string
  category: string
  description: string
}

/* ─── Party Archetype types ─── */

export interface Party {
  id: string
  kind: 'person' | 'group' | 'project'
  name: string
  user_id?: string
  is_system: boolean
  created_at: string
  updated_at: string
}

export interface PartyRelationship {
  id: string
  from_party_id: string
  from_role: string
  to_party_id: string
  to_role: string
  relationship_name: string
}

export interface LoginResponse {
  token: string
  user: User
}

/* ─── Token management (in-memory only) ─── */

let authToken: string | null = null

export function setToken(token: string | null) { authToken = token }
export function getToken() { return authToken }

/* ─── Base request helper ─── */

/** Handles 401 responses consistently: clears the token and redirects to /login. */
function handleUnauthorized(): never {
  authToken = null
  if (window.location.pathname !== '/login') {
    window.location.href = '/login'
  }
  throw new Error('Session expired')
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    ...((init?.headers as Record<string, string>) || {}),
  }
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`
  }
  if (init?.body && typeof init.body === 'string') {
    headers['Content-Type'] = 'application/json'
  }
  const res = await fetch(BASE + path, { ...init, headers })
  if (res.status === 401 && authToken) {
    handleUnauthorized()
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error ?? res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export function listCatalog(filter: ListFilter = {}): Promise<CatalogEntry[]> {
  const params = new URLSearchParams()
  if (filter.state) params.set('state', filter.state)
  else if (filter.status) params.set('state', filter.status) // backward compat
  if (filter.protocol) params.set('protocol', filter.protocol)
  if (filter.source) params.set('source', filter.source)
  if (filter.team) params.set('team', filter.team)
  if (filter.q) params.set('q', filter.q)
  if (filter.categories) params.set('categories', filter.categories)
  if (filter.limit !== undefined) params.set('limit', String(filter.limit))
  if (filter.offset !== undefined) params.set('offset', String(filter.offset))
  if (filter.sort) params.set('sort', filter.sort)
  if (filter.project) params.set('project', filter.project)
  const qs = params.toString()
  return request<CatalogEntry[]>(`/catalog${qs ? '?' + qs : ''}`)
}

export function getEntry(id: string): Promise<CatalogEntry> {
  return request<CatalogEntry>(`/catalog/${id}`)
}

export async function getRawCard(id: string): Promise<{ data: string; contentType: string; fetchedAt: string; truncated: boolean }> {
  const headers: Record<string, string> = {}
  if (authToken) headers['Authorization'] = `Bearer ${authToken}`
  const res = await fetch(`${BASE}/catalog/${id}/card`, { headers })
  if (res.status === 401 && authToken) handleUnauthorized()
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error ?? res.statusText)
  }
  const data = await res.text()
  return {
    data,
    contentType: res.headers.get('Content-Type') ?? 'application/json',
    fetchedAt: res.headers.get('X-Raw-Card-Fetched-At') ?? '',
    truncated: res.headers.get('X-Raw-Card-Truncated') === 'true',
  }
}

export function deleteEntry(id: string): Promise<void> {
  return request<void>(`/catalog/${id}`, { method: 'DELETE' })
}

export function getStats(): Promise<Stats> {
  return request<Stats>('/stats')
}

export async function validateAgentCard(cardJson: string): Promise<ValidationResult> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`
  }
  const res = await fetch(BASE + '/catalog/validate', {
    method: 'POST',
    headers,
    body: cardJson,
  })
  // Mirror the 401 handling from the shared request() helper.
  if (res.status === 401 && authToken) {
    handleUnauthorized()
  }
  // Validation endpoint returns 200 (valid) or 422 (invalid) — both are valid responses.
  if (res.status === 200 || res.status === 422) {
    return res.json() as Promise<ValidationResult>
  }
  const body = await res.json().catch(() => ({ error: res.statusText }))
  throw new Error(body.error ?? res.statusText)
}

export function createAgentFromCard(cardJson: string): Promise<CatalogEntry> {
  return request<CatalogEntry>('/catalog/register', {
    method: 'POST',
    body: cardJson,
  })
}

export interface ImportCardRequest {
  url: string
  protocol?: 'a2a' | 'mcp' | 'a2ui'
}

export function importCardFromURL(req: ImportCardRequest): Promise<CatalogEntry> {
  return request<CatalogEntry>('/catalog/import', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

export function patchLifecycle(id: string, state: LifecycleState): Promise<CatalogEntry> {
  return request<CatalogEntry>(`/catalog/${id}/lifecycle`, {
    method: 'PATCH',
    body: JSON.stringify({ state }),
  })
}

export function postProbe(id: string): Promise<Health> {
  return request<Health>(`/catalog/${id}/probe`, { method: 'POST' })
}

/* ─── Auth API ─── */

export function login(username: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

export function logout(): Promise<void> {
  return request<void>('/auth/logout', { method: 'POST' })
}

export function getMe(): Promise<User> {
  return request<User>('/auth/me')
}

export function refreshToken(): Promise<{ token: string }> {
  return request<{ token: string }>('/auth/refresh', { method: 'POST' })
}

export function changePassword(current_password: string, new_password: string): Promise<void> {
  return request<void>('/auth/password', {
    method: 'PUT',
    body: JSON.stringify({ current_password, new_password }),
  })
}

/* ─── Users API ─── */

export function listUsers(): Promise<User[]> {
  return request<User[]>('/users')
}

export function createUser(data: { username: string; email?: string; display_name?: string; password: string; role_id: string }): Promise<User> {
  return request<User>('/users', { method: 'POST', body: JSON.stringify(data) })
}

export function getUser(id: string): Promise<User> {
  return request<User>(`/users/${id}`)
}

export function updateUser(id: string, data: Partial<User>): Promise<User> {
  return request<User>(`/users/${id}`, { method: 'PUT', body: JSON.stringify(data) })
}

export function deleteUser(id: string): Promise<void> {
  return request<void>(`/users/${id}`, { method: 'DELETE' })
}

/* ─── Roles API ─── */

export function listRoles(): Promise<Role[]> {
  return request<Role[]>('/roles')
}

export function createRole(data: { name: string; description?: string; permissions: string[] }): Promise<Role> {
  return request<Role>('/roles', { method: 'POST', body: JSON.stringify(data) })
}

export function updateRole(id: string, data: Partial<Role>): Promise<Role> {
  return request<Role>(`/roles/${id}`, { method: 'PUT', body: JSON.stringify(data) })
}

export function deleteRole(id: string): Promise<void> {
  return request<void>(`/roles/${id}`, { method: 'DELETE' })
}

/* ─── Settings API ─── */

export function getSettings(): Promise<Setting[]> {
  return request<Setting[]>('/settings')
}

export function getSettingsByCategory(category: string): Promise<Setting[]> {
  return request<Setting[]>(`/settings/${category}`)
}

export function updateSettings(data: Record<string, string>): Promise<void> {
  return request<void>('/settings', { method: 'PUT', body: JSON.stringify(data) })
}

/* ─── Capabilities API ─── */

export async function listCapabilities(filter: {
  q?: string
  kind?: string
  limit?: number
  offset?: number
  sort?: string
}): Promise<CapabilityListResult> {
  const params = new URLSearchParams()
  if (filter.q) params.set('q', filter.q)
  if (filter.kind) params.set('kind', filter.kind)
  if (filter.limit) params.set('limit', filter.limit.toString())
  if (filter.offset) params.set('offset', filter.offset.toString())
  if (filter.sort) params.set('sort', filter.sort)

  const queryString = params.toString()
  const url = `/capabilities${queryString ? '?' + queryString : ''}`

  return request<CapabilityListResult>(url, {
    method: 'GET',
  })
}

export async function getCapabilityAgents(
  kind: string,
  name: string
): Promise<CapabilityDetailResponse> {
  const key = encodeURIComponent(`${kind}::${name}`)
  return request<CapabilityDetailResponse>(`/capabilities/${key}`, {
    method: 'GET',
  })
}

/* ─── Groups API ─── */

export function listGroups(): Promise<Party[]> {
  return request<Party[]>('/groups')
}

export function getGroup(id: string): Promise<Party> {
  return request<Party>(`/groups/${id}`)
}

export function createGroup(data: { name: string }): Promise<Party> {
  return request<Party>('/groups', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function deleteGroup(id: string): Promise<void> {
  return request<void>(`/groups/${id}`, { method: 'DELETE' })
}

export function listGroupMembers(id: string): Promise<PartyRelationship[]> {
  return request<PartyRelationship[]>(`/groups/${id}/members`)
}

export function addGroupMember(groupId: string, partyId: string): Promise<void> {
  return request<void>(`/groups/${groupId}/members`, {
    method: 'POST',
    body: JSON.stringify({ party_id: partyId, role: 'member' }),
  })
}

export function removeGroupMember(groupId: string, memberPartyId: string): Promise<void> {
  return request<void>(`/groups/${groupId}/members/${memberPartyId}`, {
    method: 'DELETE',
  })
}

/* ─── Projects API ─── */

export function listProjects(): Promise<Party[]> {
  return request<Party[]>('/projects')
}

export function getProject(id: string): Promise<Party> {
  return request<Party>(`/projects/${id}`)
}

export function createProject(data: { name: string }): Promise<Party> {
  return request<Party>('/projects', { method: 'POST', body: JSON.stringify(data) })
}

export function deleteProject(id: string): Promise<void> {
  return request<void>(`/projects/${id}`, { method: 'DELETE' })
}

export function listProjectMembers(id: string): Promise<PartyRelationship[]> {
  return request<PartyRelationship[]>(`/projects/${id}/members`)
}

export function addProjectMember(projectId: string, partyId: string, role: string): Promise<void> {
  return request<void>(`/projects/${projectId}/members`, {
    method: 'POST',
    body: JSON.stringify({ party_id: partyId, role }),
  })
}

export function removeProjectMember(projectId: string, memberPartyId: string): Promise<void> {
  return request<void>(`/projects/${projectId}/members/${memberPartyId}`, { method: 'DELETE' })
}

export function updateProjectMemberRole(projectId: string, memberPartyId: string, role: string): Promise<void> {
  return request<void>(`/projects/${projectId}/members/${memberPartyId}`, {
    method: 'PATCH',
    body: JSON.stringify({ role }),
  })
}

/* ─── Catalog × Project assignment ─── */

export function assignEntryToProject(entryId: string, projectId: string): Promise<void> {
  return request<void>(`/catalog/${entryId}/projects`, {
    method: 'POST',
    body: JSON.stringify({ project_id: projectId }),
  })
}

export function removeEntryFromProject(entryId: string, projectId: string): Promise<void> {
  return request<void>(`/catalog/${entryId}/projects/${projectId}`, { method: 'DELETE' })
}

export function listEntryProjects(entryId: string): Promise<Party[]> {
  return request<Party[]>(`/catalog/${entryId}/projects`)
}

/* ─── Parties API ─── */

export function listParties(kind?: 'person' | 'group' | 'project'): Promise<Party[]> {
  const qs = kind ? `?kind=${kind}` : ''
  return request<Party[]>(`/parties${qs}`)
}

/* ─── My Projects ─── */

export interface UserProjectMembership {
  project: Party
  role: string
}

export function getMyProjects(): Promise<UserProjectMembership[]> {
  return request<UserProjectMembership[]>('/auth/me/projects')
}

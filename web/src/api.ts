import type { CatalogEntry, ListFilter, Stats } from './types'

const BASE = '/api/v1'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, init)
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error ?? res.statusText)
  }
  return res.json() as Promise<T>
}

export function listCatalog(filter: ListFilter = {}): Promise<CatalogEntry[]> {
  const params = new URLSearchParams()
  if (filter.protocol) params.set('protocol', filter.protocol)
  if (filter.status) params.set('status', filter.status)
  if (filter.source) params.set('source', filter.source)
  if (filter.team) params.set('team', filter.team)
  if (filter.q) params.set('q', filter.q)
  if (filter.categories) params.set('categories', filter.categories)
  if (filter.limit !== undefined) params.set('limit', String(filter.limit))
  if (filter.offset !== undefined) params.set('offset', String(filter.offset))
  const qs = params.toString()
  return request<CatalogEntry[]>(`/catalog${qs ? '?' + qs : ''}`)
}

export function getEntry(id: string): Promise<CatalogEntry> {
  return request<CatalogEntry>(`/catalog/${id}`)
}

export function deleteEntry(id: string): Promise<void> {
  return request<void>(`/catalog/${id}`, { method: 'DELETE' })
}

export function getStats(): Promise<Stats> {
  return request<Stats>('/stats')
}

export interface Skill {
  name: string
  description: string
  tags?: string[]
  input_modes?: string[]
  output_modes?: string[]
}

export type Protocol = 'a2a' | 'mcp' | 'a2ui'
export type Status = 'healthy' | 'degraded' | 'down' | 'unknown'
export type SourceType = 'k8s' | 'config' | 'push' | 'upstream'

export interface Provider {
  organization: string
  team?: string
  url?: string
}

export interface Validity {
  from?: string
  to?: string
  last_seen: string
}

export interface CatalogEntry {
  id: string
  display_name: string
  description: string
  protocol: Protocol
  endpoint: string
  version: string
  status: Status
  source: SourceType
  provider?: Provider
  categories?: string[]
  skills?: Skill[]
  validity: Validity
  raw_card?: unknown
  metadata?: Record<string, string>
  created_at: string
  updated_at: string
}

export interface Stats {
  total: number
  by_status: Record<string, number>
  by_source: Record<string, number>
}

export interface ListFilter {
  protocol?: Protocol
  status?: Status
  source?: SourceType
  team?: string
  q?: string
  categories?: string
  limit?: number
  offset?: number
}

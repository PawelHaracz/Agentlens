export interface Skill {
  name: string
  description: string
  input_modes?: string[]
  output_modes?: string[]
}

export type Protocol = 'a2a' | 'mcp' | 'a2ui'
export type Status = 'healthy' | 'degraded' | 'down' | 'unknown'
export type SourceType = 'k8s' | 'config' | 'push' | 'upstream'

export interface Agent {
  id: string
  name: string
  description: string
  protocol: Protocol
  endpoint: string
  version: string
  status: Status
  source: SourceType
  namespace?: string
  team?: string
  tags?: string[]
  skills?: Skill[]
  raw_card?: unknown
  last_seen: string
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
  tags?: string
  limit?: number
  offset?: number
}

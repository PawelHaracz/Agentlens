export interface Capability {
  kind: string
  name: string
  description?: string
  [key: string]: unknown  // protocol-specific properties
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
  agent_type_id: string
  provider?: Provider
  categories?: string[]
  capabilities?: Capability[]
  validity: Validity
  raw_definition?: unknown
  spec_version?: string
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

export interface ValidationError {
  field: string
  message: string
}

export interface ValidationPreview {
  display_name: string
  description: string
  protocol: string
  spec_version?: string
  [key: string]: unknown  // protocol-specific preview fields
}

export interface ValidationResult {
  valid: boolean
  spec_version: string
  errors: ValidationError[]
  warnings: string[]
  preview?: ValidationPreview
}

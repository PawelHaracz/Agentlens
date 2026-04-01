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
  spec_version?: string
  typed_meta?: TypedMeta[]
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

export interface TypedMeta {
  kind: string
  [key: string]: unknown
}

export interface A2AExtensionMeta extends TypedMeta {
  kind: 'a2a.extension'
  uri: string
  required: boolean
}

export interface A2ASecuritySchemeMeta extends TypedMeta {
  kind: 'a2a.security_scheme'
  type: string
  method?: string
  name?: string
}

export interface A2AInterfaceMeta extends TypedMeta {
  kind: 'a2a.interface'
  url: string
  binding?: string
}

export interface ValidationError {
  field: string
  message: string
}

export interface ValidationPreview {
  display_name: string
  description: string
  protocol: string
  spec_version: string
  skills_count: number
  extensions_count: number
  security_schemes: string[]
  interfaces: string[]
}

export interface ValidationResult {
  valid: boolean
  spec_version: string
  errors: ValidationError[]
  warnings: string[]
  preview?: ValidationPreview
}

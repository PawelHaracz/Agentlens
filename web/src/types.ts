export interface Capability {
  kind: string
  name: string
  description?: string
  [key: string]: unknown  // protocol-specific properties
}

export interface AuthSummary {
  types: string[]
  label: string
  required: boolean
}

export interface OAuthFlow {
  flow_type: string
  authorization_url?: string
  token_url?: string
  refresh_url?: string
  device_auth_url?: string
  scopes?: Record<string, string>
  deprecated?: boolean
}

export interface SecurityScheme {
  kind?: string
  scheme_name: string
  type: string
  description?: string
  http_scheme?: string
  bearer_format?: string
  api_key_location?: string
  api_key_name?: string
  oauth_flows?: OAuthFlow[]
  oauth2_metadata_url?: string
  openid_connect_url?: string
}

export interface SecurityRequirement {
  kind?: string
  schemes: Record<string, string[]>
  skill_ref?: string
}

export interface SecurityDetailView {
  security_schemes: SecurityScheme[]
  security_requirements: SecurityRequirement[]
}

export type Protocol = 'a2a' | 'mcp' | 'a2ui'
export type LifecycleState = 'registered' | 'active' | 'degraded' | 'offline' | 'deprecated'
// Status is an alias for backward compatibility
export type Status = LifecycleState
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

export interface Health {
  state: LifecycleState
  lastProbedAt: string | null
  lastSuccessAt: string | null
  latencyMs: number
  consecutiveFailures: number
  lastError: string
}

export interface CatalogEntry {
  id: string
  display_name: string
  description: string
  protocol: Protocol
  endpoint: string
  version: string
  status: LifecycleState
  source: SourceType
  agent_type_id: string
  provider?: Provider
  categories?: string[]
  capabilities?: Capability[]
  validity: Validity
  health: Health
  raw_definition?: unknown
  spec_version?: string
  metadata?: Record<string, string>
  auth_summary?: AuthSummary
  security_detail?: SecurityDetailView
  created_at: string
  updated_at: string
}

export interface Stats {
  total: number
  by_status: Record<string, number>
  by_source: Record<string, number>
}

export interface ListFilter {
  state?: string        // comma-separated lifecycle states (preferred, new)
  protocol?: Protocol
  status?: LifecycleState  // single status backward compat
  source?: SourceType
  team?: string
  q?: string
  categories?: string
  limit?: number
  offset?: number
  sort?: 'lastSuccessAt_desc' | 'displayName_asc' | 'createdAt_desc'
}

export interface SearchMatch {
  field: string
  snippet: string
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

export interface CapabilityInstance {
  kind: string
  name: string
  description: string
  tags: string[] | null
  input_modes: string[] | null
  output_modes: string[] | null
  agent_id: string
  agent_name: string
  protocol: string
  status: string
  spec_version: string
  provider_org: string | null
  provider_url: string | null
  health_state: string
  latency_ms: number
}

export interface CapabilityListResult {
  total: number
  items: CapabilityInstance[]
}

export interface CapabilityDetailResponse {
  capability: {
    kind: string
    name: string
  }
  agents: CapabilityAgentDTO[]
}

export interface CapabilityAgentDTO {
  id: string
  display_name: string
  protocol: string
  provider: { organization: string; url: string } | null
  health: { state: string; latencyMs: number; [key: string]: unknown }
  spec_version: string
  status: string
  capability_snippet: Record<string, unknown>
}

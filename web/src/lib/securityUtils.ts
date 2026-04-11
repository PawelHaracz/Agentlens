// Types matching the A2ASecurityScheme capability JSON shape.
// These are the single source of truth — re-exported from @/types for convenience.

export interface AuthSummary {
  types: string[]
  label: string
  required: boolean
}

export interface SecurityScheme {
  kind?: string
  scheme_name: string
  type: string
  description?: string
  // http fields
  http_scheme?: string
  bearer_format?: string
  // apiKey fields
  api_key_location?: string
  api_key_name?: string
  // oauth2 fields
  oauth_flows?: OAuthFlow[]
  oauth2_metadata_url?: string
  // openIdConnect fields
  openid_connect_url?: string
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

export interface SecurityRequirement {
  kind?: string
  schemes: Record<string, string[]>
  skill_ref?: string
}

export interface SecurityDetailView {
  security_schemes: SecurityScheme[]
  security_requirements: SecurityRequirement[]
}

export function buildAuthSummaryLabel(types: string[]): string {
  if (types.length === 0) {
    return 'Open (no auth)'
  }

  const names = types.map((t) => {
    if (t === 'http:Bearer') return 'Bearer JWT'
    if (t === 'http:Basic') return 'Basic Auth'
    if (t === 'apiKey') return 'API Key'
    if (t === 'oauth2') return 'OAuth 2.0'
    if (t === 'openIdConnect') return 'OIDC'
    if (t === 'mutualTls') return 'mTLS'
    return t
  })

  const label = names.join(' + ')
  if (label.length > 40) {
    return label.substring(0, 37) + '...'
  }

  return label
}

export function generateCurlRecipe(
  endpoint: string,
  requirements: SecurityRequirement[],
  schemes: SecurityScheme[]
): string {
  if (requirements.length === 0) {
    return `curl ${endpoint}`
  }

  const firstReq = requirements[0]
  const schemeNames = Object.keys(firstReq.schemes)
  const schemeMap = new Map(schemes.map((s) => [s.scheme_name, s]))
  const headers: string[] = []

  for (const schemeName of schemeNames) {
    const scheme = schemeMap.get(schemeName)
    if (!scheme) continue

    if (scheme.type === 'http' && scheme.http_scheme === 'Bearer') {
      headers.push('-H "Authorization: Bearer <token>"')
    } else if (scheme.type === 'http' && scheme.http_scheme === 'Basic') {
      headers.push('-H "Authorization: Basic <credentials>"')
    } else if (scheme.type === 'apiKey' && scheme.api_key_location === 'header' && scheme.api_key_name) {
      headers.push(`-H "${scheme.api_key_name}: <key>"`)
    }
  }

  const headerStr = headers.join(' \\\n  ')
  if (headers.length === 0) {
    return `curl ${endpoint}`
  }
  return `curl ${headerStr} \\\n  ${endpoint}`
}

export function formatScopesLabel(scopes: string[]): string {
  return scopes.join(', ')
}

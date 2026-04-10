import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { KeyRound, Key, ShieldCheck, Fingerprint, Lock } from 'lucide-react'
import type { SecurityScheme } from '@/lib/securityUtils'

interface SchemeCardProps {
  scheme: SecurityScheme
}

export function SchemeCard({ scheme }: SchemeCardProps) {
  const { type, scheme_name } = scheme

  const Icon =
    type === 'http'
      ? KeyRound
      : type === 'apiKey'
        ? Key
        : type === 'oauth2'
          ? ShieldCheck
          : type === 'openIdConnect'
            ? Fingerprint
            : Lock

  let badgeLabel = ''
  if (type === 'http') {
    badgeLabel = scheme.bearer_format
      ? `${scheme.http_scheme} ${scheme.bearer_format}`
      : scheme.http_scheme || 'HTTP Auth'
  } else if (type === 'apiKey') {
    badgeLabel = 'API Key'
  } else if (type === 'oauth2') {
    badgeLabel = 'OAuth 2.0'
  } else if (type === 'openIdConnect') {
    badgeLabel = 'OIDC'
  } else if (type === 'mutualTls') {
    badgeLabel = 'mTLS'
  } else {
    badgeLabel = type || 'Unknown'
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Icon className="h-5 w-5" />
          <CardTitle>{scheme_name}</CardTitle>
          <Badge variant="outline">{badgeLabel}</Badge>
        </div>
        {scheme.description && <CardDescription>{scheme.description}</CardDescription>}
      </CardHeader>
      <CardContent>
        {type === 'http' && scheme.http_scheme === 'Bearer' && (
          <p className="text-sm text-muted-foreground">
            {scheme.bearer_format === 'JWT'
              ? 'Expects a JWT in the Authorization header'
              : 'Expects a Bearer token in the Authorization header'}
          </p>
        )}

        {type === 'apiKey' && (
          <div className="space-y-2">
            <p className="text-sm">
              Location: <code>{scheme.api_key_location}</code> / Name:{' '}
              <code>{scheme.api_key_name}</code>
            </p>
          </div>
        )}

        {type === 'oauth2' && (
          <div className="space-y-4">
            {scheme.oauth2_metadata_url && (
              <a
                href={scheme.oauth2_metadata_url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm text-primary underline"
              >
                OAuth 2.0 Metadata
              </a>
            )}
            {scheme.oauth_flows
              ?.filter((f) => !f.deprecated)
              .map((flow, i) => (
                <div key={i} className="border-l-2 pl-3">
                  <Badge variant="secondary" className="mb-2">
                    {formatFlowType(flow.flow_type)}
                  </Badge>
                  {flow.authorization_url && (
                    <div className="text-sm">
                      <span className="font-medium">Authorization URL:</span>{' '}
                      <a href={flow.authorization_url} target="_blank" rel="noopener noreferrer" className="text-primary underline">
                        {flow.authorization_url}
                      </a>
                    </div>
                  )}
                  {flow.token_url && (
                    <div className="text-sm">
                      <span className="font-medium">Token URL:</span>{' '}
                      <a href={flow.token_url} target="_blank" rel="noopener noreferrer" className="text-primary underline">
                        {flow.token_url}
                      </a>
                    </div>
                  )}
                  {flow.device_auth_url && (
                    <div className="text-sm">
                      <span className="font-medium">Device Authorization URL:</span>{' '}
                      <a href={flow.device_auth_url} target="_blank" rel="noopener noreferrer" className="text-primary underline">
                        {flow.device_auth_url}
                      </a>
                    </div>
                  )}
                  {flow.scopes && Object.keys(flow.scopes).length > 0 && (
                    <div className="mt-2">
                      <div className="text-sm font-medium mb-1">Scopes:</div>
                      <table className="text-sm w-full">
                        <tbody>
                          {Object.entries(flow.scopes).map(([scope, desc]) => (
                            <tr key={scope}>
                              <td className="font-mono pr-2">{scope}</td>
                              <td className="text-muted-foreground">{desc}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
              ))}
          </div>
        )}

        {type === 'openIdConnect' && scheme.openid_connect_url && (
          <a
            href={scheme.openid_connect_url}
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm text-primary underline"
          >
            OpenID Connect Discovery
          </a>
        )}

        {type === 'mutualTls' && (
          <p className="text-sm text-muted-foreground">
            This agent requires mutual TLS. Configure your client certificate before connecting.
          </p>
        )}
      </CardContent>
    </Card>
  )
}

function formatFlowType(flowType: string): string {
  const map: Record<string, string> = {
    authorizationCode: 'Authorization Code',
    clientCredentials: 'Client Credentials',
    deviceCode: 'Device Code',
  }
  return map[flowType] || flowType
}

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { SchemeCard } from './SchemeCard'
import { SecurityRequirementsBanner } from './SecurityRequirementsBanner'
import { ConnectionRecipe } from './ConnectionRecipe'
import type { SecurityDetailView } from '@/lib/securityUtils'

interface AuthenticationSectionProps {
  protocol: string
  securityDetail?: SecurityDetailView
  endpoint?: string
}

export function AuthenticationSection({
  protocol,
  securityDetail,
  endpoint,
}: AuthenticationSectionProps) {
  if (protocol === 'mcp') {
    return (
      <Card data-testid="authentication-section">
        <CardHeader>
          <CardTitle>Authentication</CardTitle>
        </CardHeader>
        <CardContent>
          <Alert>
            <AlertDescription>
              MCP servers declare authentication at the transport level, not in the server card.
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    )
  }

  if (
    !securityDetail ||
    (!securityDetail.security_schemes?.length && !securityDetail.security_requirements?.length)
  ) {
    return (
      <Card data-testid="authentication-section">
        <CardHeader>
          <CardTitle>Authentication</CardTitle>
        </CardHeader>
        <CardContent>
          <Alert>
            <AlertDescription>
              This agent does not declare any authentication requirements.
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    )
  }

  const { security_schemes = [], security_requirements } = securityDetail
  const topLevelRequirements = (security_requirements ?? []).filter((r) => !r.skill_ref)

  return (
    <Card data-testid="authentication-section">
      <CardHeader>
        <CardTitle>Authentication</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {security_requirements && security_requirements.length > 0 && (
          <SecurityRequirementsBanner requirements={security_requirements} />
        )}

        <div className="space-y-3">
          {security_schemes.map((scheme) => (
            <SchemeCard key={scheme.scheme_name} scheme={scheme} />
          ))}
        </div>

        {endpoint && topLevelRequirements.length > 0 && (
          <ConnectionRecipe
            endpoint={endpoint}
            requirements={topLevelRequirements}
            schemes={security_schemes}
          />
        )}
      </CardContent>
    </Card>
  )
}

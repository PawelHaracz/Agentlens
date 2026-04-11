import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { ShieldCheck } from 'lucide-react'
import type { SecurityRequirement } from '@/lib/securityUtils'

interface SecurityRequirementsBannerProps {
  requirements: SecurityRequirement[]
}

export function SecurityRequirementsBanner({ requirements }: SecurityRequirementsBannerProps) {
  const topLevel = requirements.filter((r) => !r.skill_ref)
  if (topLevel.length === 0) {
    return null
  }

  const isMultiple = topLevel.length > 1

  return (
    <Alert data-testid="security-requirements-banner">
      <ShieldCheck className="h-4 w-4" />
      <AlertTitle>Required to connect</AlertTitle>
      <AlertDescription>
        {isMultiple && <p className="mb-2">Any of the following combinations:</p>}
        <ul className="list-disc pl-5 space-y-1">
          {topLevel.map((req) => {
            const sortedSchemes = Object.entries(req.schemes).sort(([a], [b]) => a.localeCompare(b))
            const reqKey = sortedSchemes.map(([k]) => k).join('+') || (req.skill_ref ?? 'req')
            return (
              <li key={reqKey}>
                {sortedSchemes.map(([schemeName, scopes], j) => (
                  <span key={schemeName}>
                    {j > 0 && ' AND '}
                    <strong>{schemeName}</strong>
                    {scopes.length > 0 && ` (scopes: ${scopes.join(', ')})`}
                  </span>
                ))}
              </li>
            )
          })}
        </ul>
      </AlertDescription>
    </Alert>
  )
}

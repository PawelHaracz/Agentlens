import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Copy } from 'lucide-react'
import type { SecurityScheme, SecurityRequirement } from '@/lib/securityUtils'
import { generateCurlRecipe } from '@/lib/securityUtils'

interface ConnectionRecipeProps {
  endpoint: string
  requirements: SecurityRequirement[]
  schemes: SecurityScheme[]
}

export function ConnectionRecipe({ endpoint, requirements, schemes }: ConnectionRecipeProps) {
  const curl = generateCurlRecipe(endpoint, requirements, schemes)

  const copyToClipboard = () => {
    navigator.clipboard.writeText(curl).catch(() => {})
  }

  return (
    <Card data-testid="connection-recipe">
      <CardHeader>
        <CardTitle>Connection Example</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="relative">
          <pre className="bg-muted p-4 rounded text-sm overflow-x-auto">
            <code>{curl}</code>
          </pre>
          <Button
            size="sm"
            variant="ghost"
            className="absolute top-2 right-2"
            aria-label="Copy connection command"
            onClick={copyToClipboard}
          >
            <Copy className="h-4 w-4" />
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

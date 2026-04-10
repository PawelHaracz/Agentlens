import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

interface AuthBadgeProps {
  label: string
  required: boolean
}

export function AuthBadge({ label, required: _required }: AuthBadgeProps) {
  const truncated = label.length > 25 ? label.substring(0, 22) + '...' : label
  const variant = label === 'Open (no auth)' ? 'secondary' : 'outline'

  if (label.length > 25) {
    return (
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger>
            <Badge variant={variant}>{truncated}</Badge>
          </TooltipTrigger>
          <TooltipContent>
            <p>{label}</p>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    )
  }

  return <Badge variant={variant}>{truncated}</Badge>
}

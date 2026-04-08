import { Badge } from '@/components/ui/badge'

interface Props {
  version?: string
}

export function SpecVersionBadge({ version }: Props) {
  if (!version) return null
  return (
    <Badge variant="outline" className="font-mono text-xs">
      {version}
    </Badge>
  )
}

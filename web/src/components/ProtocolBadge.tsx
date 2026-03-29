import type { Protocol } from '../types'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

const protocolStyles: Record<Protocol, string> = {
  a2a: 'bg-blue-100 text-blue-800 hover:bg-blue-100 border-blue-200',
  mcp: 'bg-green-100 text-green-800 hover:bg-green-100 border-green-200',
  a2ui: 'bg-purple-100 text-purple-800 hover:bg-purple-100 border-purple-200',
}

export default function ProtocolBadge({ protocol }: { protocol: Protocol }) {
  return (
    <Badge variant="outline" className={cn('uppercase', protocolStyles[protocol])}>
      {protocol}
    </Badge>
  )
}

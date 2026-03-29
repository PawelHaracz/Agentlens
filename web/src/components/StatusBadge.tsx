import type { Status } from '../types'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

const statusConfig: Record<Status, { variant: 'default' | 'secondary' | 'destructive' | 'outline'; className: string }> = {
  healthy: { variant: 'default', className: 'bg-green-100 text-green-800 hover:bg-green-100 border-green-200' },
  degraded: { variant: 'outline', className: 'bg-yellow-50 text-yellow-800 border-yellow-300' },
  down: { variant: 'destructive', className: '' },
  unknown: { variant: 'secondary', className: '' },
}

export default function StatusBadge({ status }: { status: Status }) {
  const config = statusConfig[status]
  return (
    <Badge variant={config.variant} className={cn(config.className)}>
      {status}
    </Badge>
  )
}

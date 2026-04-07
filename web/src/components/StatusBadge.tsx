import type { LifecycleState } from '../types'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

interface StatusBadgeProps {
  status: LifecycleState
  latencyMs?: number
  lastSeenAt?: string
}

const lifecycleConfig: Record<LifecycleState, {
  variant: 'default' | 'secondary' | 'destructive' | 'outline'
  className: string
  label: string
  pendingTooltip?: string
}> = {
  active:     { variant: 'default',     className: 'bg-green-100 text-green-800 hover:bg-green-100 border-green-200', label: 'Active' },
  degraded:   { variant: 'outline',     className: 'bg-yellow-50 text-yellow-800 border-yellow-300',                  label: 'Degraded' },
  offline:    { variant: 'destructive', className: '',                                                                  label: 'Offline' },
  registered: { variant: 'secondary',   className: '',  label: 'Pending', pendingTooltip: 'Will be probed within next interval' },
  deprecated: { variant: 'outline',     className: 'text-slate-500 border-slate-300',                                  label: 'Deprecated' },
}

function relativeTime(isoStr: string): string {
  const diff = Math.floor((Date.now() - new Date(isoStr).getTime()) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  return `${Math.floor(diff / 3600)}h ago`
}

export default function StatusBadge({ status, latencyMs, lastSeenAt }: StatusBadgeProps) {
  const config = lifecycleConfig[status] ?? lifecycleConfig.registered
  const showLatency = (status === 'active' || status === 'degraded') && latencyMs != null && latencyMs > 0
  const tooltipText = lastSeenAt
    ? new Date(lastSeenAt).toUTCString()
    : config.pendingTooltip

  return (
    <TooltipProvider>
      <div className="flex items-center gap-2">
        <Tooltip>
          <TooltipTrigger asChild>
            <Badge variant={config.variant} className={cn(config.className, 'cursor-default')}>
              {config.label}
            </Badge>
          </TooltipTrigger>
          {tooltipText && (
            <TooltipContent>
              <p>{tooltipText}</p>
            </TooltipContent>
          )}
        </Tooltip>
        {showLatency && (
          <span className="text-xs text-muted-foreground">{latencyMs} ms</span>
        )}
        {lastSeenAt && (
          <span className="text-xs text-muted-foreground">{relativeTime(lastSeenAt)}</span>
        )}
      </div>
    </TooltipProvider>
  )
}

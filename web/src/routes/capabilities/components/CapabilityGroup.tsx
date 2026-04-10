import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import StatusBadge from '@/components/StatusBadge'
import ProtocolBadge from '@/components/ProtocolBadge'
import type { CapabilityInstance } from '@/types'
import type { LifecycleState, Protocol } from '@/types'

interface CapabilityGroupProps {
  kind: string
  name: string
  items: CapabilityInstance[]
}

function kindToLabel(kind: string): string {
  const map: Record<string, string> = {
    'a2a.skill': 'A2A Skill',
    'mcp.tool': 'MCP Tool',
    'mcp.resource': 'MCP Resource',
    'mcp.prompt': 'MCP Prompt',
  }
  return map[kind] || kind
}

export function CapabilityGroup({ kind, name, items }: CapabilityGroupProps) {
  const [isOpen, setIsOpen] = useState(false)

  const firstItem = items[0]
  const description = firstItem?.description || ''
  const tags = firstItem?.tags || []
  const visibleTags = tags.slice(0, 5)
  const remainingTags = tags.length - 5

  const detailURL = `/catalog/capabilities/${encodeURIComponent(kind + '::' + name)}`

  return (
    <div className="border rounded-lg">
      {/* Header */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="w-full px-4 py-3 flex items-center gap-3 hover:bg-muted/50 transition-colors text-left"
      >
        {isOpen ? (
          <ChevronDown className="h-4 w-4 text-muted-foreground flex-shrink-0" />
        ) : (
          <ChevronRight className="h-4 w-4 text-muted-foreground flex-shrink-0" />
        )}

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <Badge variant="outline">{kindToLabel(kind)}</Badge>
            <h3 className="font-semibold truncate">{name}</h3>
          </div>

          {description && (
            <p className="text-sm text-muted-foreground line-clamp-1">{description}</p>
          )}

          {visibleTags.length > 0 && (
            <div className="flex gap-1 mt-2">
              {visibleTags.map((tag) => (
                <Badge key={tag} variant="secondary" className="text-xs">
                  {tag}
                </Badge>
              ))}
              {remainingTags > 0 && (
                <Badge variant="secondary" className="text-xs">
                  +{remainingTags} more
                </Badge>
              )}
            </div>
          )}
        </div>

        <div className="text-sm text-muted-foreground flex-shrink-0">
          {items.length} {items.length === 1 ? 'agent' : 'agents'}
        </div>
      </button>

      {/* Expanded content */}
      {isOpen && (
        <div className="border-t px-4 py-3 space-y-2">
          {items.map((item) => (
            <div
              key={item.agent_id}
              className="flex items-center gap-3 p-2 rounded hover:bg-muted/50"
            >
              <ProtocolBadge protocol={item.protocol as Protocol} />
              <Link
                to={`/catalog/${item.agent_id}`}
                className="font-medium hover:underline flex-1 truncate"
              >
                {item.agent_name}
              </Link>
              <StatusBadge
                status={item.status as LifecycleState}
                latencyMs={item.latency_ms}
              />
              {item.provider_org && (
                <span className="text-sm text-muted-foreground truncate">{item.provider_org}</span>
              )}
            </div>
          ))}

          <div className="pt-2">
            <Button variant="outline" size="sm" asChild>
              <Link to={detailURL}>View all →</Link>
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

import { useNavigate } from 'react-router-dom'
import { ExternalLink } from 'lucide-react'
import type { CatalogEntry } from '../../../types'
import StatusBadge from '../../../components/StatusBadge'
import ProtocolBadge from '../../../components/ProtocolBadge'
import { TableRow, TableCell } from '../../../components/ui/table'
import { SpecVersionBadge } from './SpecVersionBadge'
import { SearchHighlight } from './SearchHighlight'

function relativeTime(iso: string | null): string {
  if (!iso) return '—'
  const ms = Date.now() - new Date(iso).getTime()
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s ago`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}

interface Props {
  entry: CatalogEntry
  searchSnippet?: string
}

export function CatalogRow({ entry, searchSnippet }: Props) {
  const navigate = useNavigate()

  const skillCount = entry.capabilities?.length ?? 0

  return (
    <TableRow
      className="cursor-pointer"
      onClick={() => navigate(`/catalog/${entry.id}`)}
      role="link"
      tabIndex={0}
      aria-label={`View details for ${entry.display_name}`}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          navigate(`/catalog/${entry.id}`)
        }
      }}
    >
      <TableCell>
        <ProtocolBadge protocol={entry.protocol} />
      </TableCell>

      <TableCell>
        <div className="font-medium">{entry.display_name}</div>
        {entry.description && (
          <p className="text-xs text-muted-foreground line-clamp-1">{entry.description}</p>
        )}
        <SearchHighlight snippet={searchSnippet} />
      </TableCell>

      <TableCell>
        {entry.provider ? (
          <div className="flex items-center gap-1">
            <span className="text-sm">{entry.provider.organization}</span>
            {entry.provider.url && (
              <a
                href={entry.provider.url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-muted-foreground hover:text-foreground"
                onClick={(e) => e.stopPropagation()}
                aria-label={`Open ${entry.provider.organization} website`}
              >
                <ExternalLink className="h-3 w-3" />
              </a>
            )}
          </div>
        ) : (
          <span className="text-muted-foreground text-sm">—</span>
        )}
      </TableCell>

      <TableCell className="text-sm text-center">
        {skillCount > 0 ? skillCount : <span className="text-muted-foreground">—</span>}
      </TableCell>

      <TableCell>
        <StatusBadge
          status={entry.status}
          latencyMs={entry.health?.latencyMs}
          lastSeenAt={entry.health?.lastSuccessAt ?? undefined}
        />
      </TableCell>

      <TableCell>
        <SpecVersionBadge version={entry.spec_version} />
      </TableCell>

      <TableCell className="text-sm text-muted-foreground">
        {relativeTime(entry.health?.lastSuccessAt ?? null)}
      </TableCell>
    </TableRow>
  )
}

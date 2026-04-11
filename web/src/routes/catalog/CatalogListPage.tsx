import { useState, useEffect, useCallback } from 'react'
import { AlertCircle, RefreshCw, SearchX, Inbox } from 'lucide-react'
import type { Stats } from '../../types'
import { getStats } from '@/api'
import { useCatalogQuery } from '../../hooks/useCatalogQuery'
import StatsBar from '../../components/StatsBar'
import RegisterAgentDialog from '../../components/RegisterAgentDialog'
import { ProtocolFilter } from './components/ProtocolFilter'
import { UnifiedSearchBox } from './components/UnifiedSearchBox'
import { CatalogRow } from './components/CatalogRow'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
} from '../../components/ui/table'
import { Skeleton } from '../../components/ui/skeleton'
import { Alert, AlertTitle, AlertDescription } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'

const SKELETON_COUNT = 8

export default function CatalogListPage() {
  const { entries, isLoading, isError, error, filter, setProtocol, setQuery, clearFilters, refetch } =
    useCatalogQuery()

  const [stats, setStats] = useState<Stats | null>(null)

  const fetchStats = useCallback(async () => {
    try {
      const s = await getStats()
      setStats(s)
    } catch {
      // stats are non-critical; silently ignore failures
    }
  }, [])

  useEffect(() => {
    fetchStats()
  }, [fetchStats])

  const handleRegistered = useCallback(() => {
    refetch()
    fetchStats()
  }, [refetch, fetchStats])

  const hasActiveFilters = Boolean(filter.protocol || filter.q)

  return (
    <div className="container mx-auto py-6 space-y-4">
      {/* Stats */}
      {stats && <StatsBar stats={stats} />}

      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-3">
        <ProtocolFilter value={filter.protocol} onChange={setProtocol} />
        <UnifiedSearchBox value={filter.q} onChange={setQuery} />
        <RegisterAgentDialog onRegistered={handleRegistered} />
      </div>

      {/* Loading skeleton */}
      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: SKELETON_COUNT }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full rounded-md" />
          ))}
        </div>
      )}

      {/* Error state */}
      {isError && !isLoading && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>Failed to load catalog</AlertTitle>
          <AlertDescription className="flex items-center gap-3 mt-1">
            <span>{error?.message ?? 'An unexpected error occurred.'}</span>
            <Button
              size="sm"
              variant="outline"
              onClick={() => refetch()}
              className="shrink-0"
            >
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {/* Empty / no-results states */}
      {!isLoading && !isError && entries !== undefined && entries.length === 0 && (
        hasActiveFilters ? (
          <div className="flex flex-col items-center justify-center py-16 text-center gap-3">
            <SearchX className="h-10 w-10 text-muted-foreground" />
            <p className="text-muted-foreground text-sm">No agents match the current filters.</p>
            <Button variant="outline" size="sm" onClick={clearFilters}>
              Clear filters
            </Button>
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center py-16 text-center gap-3">
            <Inbox className="h-10 w-10 text-muted-foreground" />
            <p className="text-muted-foreground text-sm">No agents registered yet.</p>
          </div>
        )
      )}

      {/* Catalog table */}
      {!isLoading && !isError && entries && entries.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Protocol</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Provider</TableHead>
              <TableHead className="text-center">Skills</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Spec</TableHead>
              <TableHead>Auth</TableHead>
              <TableHead>Last seen</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map(entry => (
              <CatalogRow key={entry.id} entry={entry} />
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

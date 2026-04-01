import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { listCatalog, getStats } from '../api'
import type { CatalogEntry, Stats, Protocol, Status } from '../types'
import StatusBadge from './StatusBadge'
import ProtocolBadge from './ProtocolBadge'
import StatsBar from './StatsBar'
import SearchBar from './SearchBar'
import RegisterAgentDialog from './RegisterAgentDialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

export default function CatalogList() {
  const [entries, setEntries] = useState<CatalogEntry[]>([])
  const [stats, setStats] = useState<Stats | null>(null)
  const [search, setSearch] = useState('')
  const [protocol, setProtocol] = useState<Protocol | 'all'>('all')
  const [status, setStatus] = useState<Status | 'all'>('all')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [a, s] = await Promise.all([
        listCatalog({
          q: search || undefined,
          protocol: protocol === 'all' ? undefined : protocol,
          status: status === 'all' ? undefined : status,
        }),
        getStats(),
      ])
      setEntries(a)
      setStats(s)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }, [search, protocol, status])

  useEffect(() => {
    load()
  }, [load])

  return (
    <div>
      {stats && <StatsBar stats={stats} />}

      <div className="flex flex-col sm:flex-row gap-2 mb-4">
        <div className="flex-1">
          <SearchBar value={search} onChange={setSearch} />
        </div>
        <Select value={protocol} onValueChange={v => setProtocol(v as Protocol | 'all')}>
          <SelectTrigger className="w-[160px]">
            <SelectValue placeholder="All protocols" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All protocols</SelectItem>
            <SelectItem value="a2a">A2A</SelectItem>
            <SelectItem value="mcp">MCP</SelectItem>
            <SelectItem value="a2ui">A2UI</SelectItem>
          </SelectContent>
        </Select>
        <Select value={status} onValueChange={v => setStatus(v as Status | 'all')}>
          <SelectTrigger className="w-[160px]">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            <SelectItem value="healthy">Healthy</SelectItem>
            <SelectItem value="degraded">Degraded</SelectItem>
            <SelectItem value="down">Down</SelectItem>
            <SelectItem value="unknown">Unknown</SelectItem>
          </SelectContent>
        </Select>
        <RegisterAgentDialog onRegistered={load} />
      </div>

      {error && (
        <Card className="border-destructive bg-destructive/10 p-4 mb-4 text-destructive text-sm">
          {error}
        </Card>
      )}

      {loading ? (
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      ) : entries.length === 0 ? (
        <div className="text-center py-12 text-muted-foreground text-sm">No catalog entries found.</div>
      ) : (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="px-4">Name</TableHead>
                <TableHead className="px-4">Protocol</TableHead>
                <TableHead className="px-4">Status</TableHead>
                <TableHead className="px-4 hidden sm:table-cell">Source</TableHead>
                <TableHead className="px-4 hidden md:table-cell">Endpoint</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.map(entry => (
                <EntryRow key={entry.id} entry={entry} />
              ))}
            </TableBody>
          </Table>
        </Card>
      )}
    </div>
  )
}

function EntryRow({ entry }: { entry: CatalogEntry }) {
  return (
    <TableRow>
      <TableCell className="px-4 py-3">
        <Link to={`/catalog/${entry.id}`} className="font-medium text-primary hover:underline">
          {entry.display_name}
        </Link>
        {entry.description && (
          <p className="text-xs text-muted-foreground mt-0.5 truncate max-w-xs">{entry.description}</p>
        )}
      </TableCell>
      <TableCell className="px-4 py-3">
        <ProtocolBadge protocol={entry.protocol} />
      </TableCell>
      <TableCell className="px-4 py-3">
        <StatusBadge status={entry.status} />
      </TableCell>
      <TableCell className="px-4 py-3 text-sm text-muted-foreground hidden sm:table-cell">{entry.source}</TableCell>
      <TableCell className="px-4 py-3 text-sm text-muted-foreground hidden md:table-cell font-mono truncate max-w-xs">{entry.endpoint}</TableCell>
    </TableRow>
  )
}

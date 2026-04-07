import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { listCatalog, getStats } from '../api'
import type { CatalogEntry, Stats, Protocol, LifecycleState } from '../types'
import StatusBadge from './StatusBadge'
import ProtocolBadge from './ProtocolBadge'
import StatsBar from './StatsBar'
import SearchBar from './SearchBar'
import RegisterAgentDialog from './RegisterAgentDialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { ChevronDown } from 'lucide-react'

const LIFECYCLE_OPTIONS: { value: LifecycleState; label: string }[] = [
  { value: 'active',     label: 'Active' },
  { value: 'degraded',   label: 'Degraded' },
  { value: 'offline',    label: 'Offline' },
  { value: 'registered', label: 'Pending' },
  { value: 'deprecated', label: 'Deprecated' },
]

export default function CatalogList() {
  const [entries, setEntries] = useState<CatalogEntry[]>([])
  const [stats, setStats] = useState<Stats | null>(null)
  const [search, setSearch] = useState('')
  const [protocol, setProtocol] = useState<Protocol | 'all'>('all')
  const [selectedStates, setSelectedStates] = useState<LifecycleState[]>([])
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
          state: selectedStates.length > 0 ? selectedStates.join(',') : undefined,
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
  }, [search, protocol, selectedStates])

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
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" className="w-[160px] justify-between">
              {selectedStates.length === 0
                ? 'All statuses'
                : `${selectedStates.length} selected`}
              <ChevronDown className="ml-2 h-4 w-4 opacity-50" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            {LIFECYCLE_OPTIONS.map(opt => (
              <DropdownMenuCheckboxItem
                key={opt.value}
                checked={selectedStates.includes(opt.value)}
                onCheckedChange={checked =>
                  setSelectedStates(prev =>
                    checked ? [...prev, opt.value] : prev.filter(s => s !== opt.value)
                  )
                }
              >
                {opt.label}
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
        <RegisterAgentDialog onRegistered={load} />
      </div>

      {error && (
        <Card className="border-destructive bg-destructive/10 p-4 mb-4 text-destructive text-sm">
          {error}
        </Card>
      )}

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
            {loading && Array.from({ length: 5 }).map((_, i) => (
              <TableRow key={i}>
                <TableCell className="px-4 py-3"><Skeleton className="h-5 w-48" /></TableCell>
                <TableCell className="px-4 py-3"><Skeleton className="h-5 w-16" /></TableCell>
                <TableCell className="px-4 py-3"><Skeleton className="h-5 w-20" /></TableCell>
                <TableCell className="px-4 py-3 hidden sm:table-cell"><Skeleton className="h-5 w-16" /></TableCell>
                <TableCell className="px-4 py-3 hidden md:table-cell"><Skeleton className="h-5 w-40" /></TableCell>
              </TableRow>
            ))}
            {!loading && entries.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                  {selectedStates.length > 0 ? (
                    <div className="space-y-2">
                      <p>No entries match the selected status filter.</p>
                      <Button variant="link" className="p-0 h-auto" onClick={() => setSelectedStates([])}>
                        Clear filters
                      </Button>
                    </div>
                  ) : (
                    <p>No catalog entries found.</p>
                  )}
                </TableCell>
              </TableRow>
            )}
            {!loading && entries.map(entry => (
              <EntryRow key={entry.id} entry={entry} />
            ))}
          </TableBody>
        </Table>
      </Card>
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
        <StatusBadge
          status={entry.status}
          latencyMs={entry.health?.latencyMs}
          lastSeenAt={entry.health?.lastSuccessAt ?? entry.validity?.last_seen}
        />
      </TableCell>
      <TableCell className="px-4 py-3 text-sm text-muted-foreground hidden sm:table-cell">{entry.source}</TableCell>
      <TableCell className="px-4 py-3 text-sm text-muted-foreground hidden md:table-cell font-mono truncate max-w-xs">{entry.endpoint}</TableCell>
    </TableRow>
  )
}

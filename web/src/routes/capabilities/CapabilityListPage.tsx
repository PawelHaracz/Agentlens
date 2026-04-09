import { useMemo } from 'react'
import { Loader2, SearchX, Inbox } from 'lucide-react'
import { useCapabilitiesQuery } from '@/hooks/useCapabilitiesQuery'
import { UnifiedSearchBox } from '@/routes/catalog/components/UnifiedSearchBox'
import { KindFilter } from './components/KindFilter'
import { CapabilityGroup } from './components/CapabilityGroup'
import { Button } from '@/components/ui/button'
import type { CapabilityInstance } from '@/types'

export default function CapabilityListPage() {
  const {
    data,
    isLoading,
    isError,
    error,
    query,
    kind,
    setQuery,
    setKind,
    clearFilters,
  } = useCapabilitiesQuery()

  // Group items by (kind, name)
  const groups = useMemo(() => {
    if (!data?.items) return []

    const map = new Map<string, CapabilityInstance[]>()
    for (const item of data.items) {
      const key = `${item.kind}::${item.name}`
      if (!map.has(key)) {
        map.set(key, [])
      }
      map.get(key)!.push(item)
    }

    // Convert to array and sort by name
    return Array.from(map.entries())
      .map(([key, items]) => {
        const [itemKind, ...nameParts] = key.split('::')
        return { kind: itemKind, name: nameParts.join('::'), items }
      })
      .sort((a, b) => a.name.localeCompare(b.name))
  }, [data?.items])

  const hasActiveFilters = Boolean(query || kind)

  if (isLoading) {
    return (
      <div className="container mx-auto py-6">
        <div className="flex items-center justify-center h-64">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </div>
    )
  }

  if (isError) {
    return (
      <div className="container mx-auto py-6">
        <div className="text-center py-12">
          <p className="text-destructive">Failed to load capabilities</p>
          <p className="text-sm text-muted-foreground mt-1">
            {error instanceof Error ? error.message : 'Unknown error'}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Capabilities</h1>
        <p className="text-muted-foreground mt-1">Discover agents by capability</p>
      </div>

      {/* Toolbar */}
      <div className="space-y-3">
        <UnifiedSearchBox
          value={query}
          onChange={setQuery}
        />
        <KindFilter value={kind} onChange={setKind} />
      </div>

      {/* Empty states */}
      {groups.length === 0 && (
        hasActiveFilters ? (
          <div className="flex flex-col items-center justify-center py-16 text-center gap-3">
            <SearchX className="h-10 w-10 text-muted-foreground" />
            <p className="text-muted-foreground text-sm">No capabilities match the current filters.</p>
            <Button variant="outline" size="sm" onClick={clearFilters}>
              Clear filters
            </Button>
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center py-16 text-center gap-3">
            <Inbox className="h-10 w-10 text-muted-foreground" />
            <p className="text-muted-foreground text-sm">No capabilities published yet.</p>
          </div>
        )
      )}

      {/* Grouped accordion list */}
      {groups.length > 0 && (
        <div className="space-y-3">
          {groups.map((group) => (
            <CapabilityGroup
              key={`${group.kind}::${group.name}`}
              kind={group.kind}
              name={group.name}
              items={group.items}
            />
          ))}
        </div>
      )}
    </div>
  )
}

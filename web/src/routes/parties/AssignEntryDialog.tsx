import { useEffect, useMemo, useState } from 'react'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import * as api from '@/api'
import type { CatalogEntry } from '@/types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAssigned: () => void
  projectId: string
  alreadyAssignedIds: Set<string>
}

export default function AssignEntryDialog({
  open, onOpenChange, onAssigned, projectId, alreadyAssignedIds,
}: Props) {
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [results, setResults] = useState<CatalogEntry[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 250)
    return () => clearTimeout(t)
  }, [query])

  useEffect(() => {
    if (!debouncedQuery) { setResults([]); return }
    api.listCatalog({ q: debouncedQuery, limit: 20 })
      .then(r => setResults(r ?? []))
      .catch(() => setResults([]))
  }, [debouncedQuery])

  const visible = useMemo(
    () => results.filter(e => !alreadyAssignedIds.has(e.id)),
    [results, alreadyAssignedIds]
  )

  const reset = () => {
    setQuery('')
    setDebouncedQuery('')
    setResults([])
    setSelectedId('')
    setError('')
    setSaving(false)
  }

  const handleSubmit = async () => {
    if (!selectedId) return
    setSaving(true)
    try {
      await api.assignEntryToProject(selectedId, projectId)
      onAssigned()
      reset()
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to assign entry')
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) reset(); onOpenChange(o) }}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Assign catalog entry</DialogTitle>
          <DialogDescription>Search the catalog, pick an entry, and assign it to this project.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}
          <Input
            placeholder="Search catalog by name…"
            value={query}
            onChange={e => setQuery(e.target.value)}
            autoFocus
          />
          <div className="max-h-80 overflow-y-auto rounded-md border">
            {visible.length === 0 && debouncedQuery && (
              <div className="p-4 text-sm text-muted-foreground">No matching entries.</div>
            )}
            {visible.map(e => (
              <button
                key={e.id}
                type="button"
                onClick={() => setSelectedId(e.id)}
                className={`w-full text-left p-3 border-b last:border-b-0 hover:bg-muted/50 ${selectedId === e.id ? 'bg-muted' : ''}`}
              >
                <div className="flex items-center justify-between">
                  <span className="font-medium">{e.display_name}</span>
                  <Badge variant="outline">{e.protocol}</Badge>
                </div>
                <div className="text-xs text-muted-foreground">{e.endpoint} · v{e.version}</div>
              </button>
            ))}
          </div>
        </div>
        <DialogFooter>
          <Button onClick={handleSubmit} disabled={saving || !selectedId}>
            {saving ? 'Assigning…' : 'Assign'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

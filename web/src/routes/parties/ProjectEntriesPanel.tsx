import { useEffect, useMemo, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import * as api from '@/api'
import type { CatalogEntry } from '@/types'
import AssignEntryDialog from './AssignEntryDialog'

interface Props {
  projectId: string
  canWrite: boolean
}

export default function ProjectEntriesPanel({ projectId, canWrite }: Props) {
  const [entries, setEntries] = useState<CatalogEntry[]>([])
  const [assignOpen, setAssignOpen] = useState(false)

  const load = () => {
    api.listCatalog({ project: projectId }).then(r => setEntries(r ?? [])).catch(() => {})
  }
  useEffect(load, [projectId])

  const assignedIds = useMemo(() => new Set(entries.map(e => e.id)), [entries])

  const handleRemove = async (e: CatalogEntry) => {
    if (!confirm(`Remove "${e.display_name}" from this project?`)) return
    try { await api.removeEntryFromProject(e.id, projectId); load() } catch { /* ignore */ }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Assigned catalog entries</h3>
        {canWrite && (
          <Button size="sm" className="gap-1" onClick={() => setAssignOpen(true)}>
            <Plus className="h-4 w-4" /> Assign entry
          </Button>
        )}
      </div>
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Protocol</TableHead>
              <TableHead>Endpoint</TableHead>
              {canWrite && <TableHead className="w-24">Actions</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map(e => (
              <TableRow key={e.id}>
                <TableCell className="font-medium">{e.display_name}</TableCell>
                <TableCell><Badge variant="outline">{e.protocol}</Badge></TableCell>
                <TableCell className="text-xs text-muted-foreground">{e.endpoint}</TableCell>
                {canWrite && (
                  <TableCell>
                    <Button variant="ghost" size="icon" onClick={() => handleRemove(e)} title="Remove">
                      <Trash2 className="h-3.5 w-3.5 text-destructive" />
                    </Button>
                  </TableCell>
                )}
              </TableRow>
            ))}
            {entries.length === 0 && (
              <TableRow>
                <TableCell colSpan={canWrite ? 4 : 3} className="text-center text-muted-foreground py-8">
                  No entries assigned yet.{canWrite && <> Click <strong>Assign entry</strong>.</>}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <AssignEntryDialog
        open={assignOpen}
        onOpenChange={setAssignOpen}
        onAssigned={load}
        projectId={projectId}
        alreadyAssignedIds={assignedIds}
      />
    </div>
  )
}

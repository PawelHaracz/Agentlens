import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Plus, Trash2 } from 'lucide-react'
import { useAuth } from '../../contexts/AuthContext'
import * as api from '@/api'
import type { Party } from '@/api'
import CreatePartyDialog from './CreatePartyDialog'
import type { PartyUIConfig } from './partyUIConfig'

interface Props {
  config: PartyUIConfig
}

export default function PartyTab({ config }: Props) {
  const { hasPermission } = useAuth()
  const navigate = useNavigate()
  const [parties, setParties] = useState<Party[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const canWrite = hasPermission(config.writePermission)

  const listFn = config.kind === 'group' ? api.listGroups : api.listProjects
  const deleteFn = config.kind === 'group' ? api.deleteGroup : api.deleteProject

  const load = () => { listFn().then(setParties).catch(() => {}) }
  useEffect(load, [listFn])

  const handleDelete = async (p: Party) => {
    if (!confirm(`Delete ${config.labels.single.toLowerCase()} "${p.name}"?`)) return
    try { await deleteFn(p.id); load() } catch { /* ignore */ }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">{config.labels.plural}</h3>
        {canWrite && (
          <Button size="sm" className="gap-1" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" /> Create {config.labels.single.toLowerCase()}
          </Button>
        )}
      </div>
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Created</TableHead>
              {canWrite && <TableHead className="w-24">Actions</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {parties.map(p => (
              <TableRow
                key={p.id}
                className="cursor-pointer hover:bg-muted/50"
                onClick={() => navigate(config.detailPath(p.id))}
              >
                <TableCell className="font-medium">{p.name}</TableCell>
                <TableCell>{new Date(p.created_at).toLocaleDateString()}</TableCell>
                {canWrite && (
                  <TableCell onClick={e => e.stopPropagation()}>
                    <Button variant="ghost" size="icon" onClick={() => handleDelete(p)} title="Delete">
                      <Trash2 className="h-3.5 w-3.5 text-destructive" />
                    </Button>
                  </TableCell>
                )}
              </TableRow>
            ))}
            {parties.length === 0 && (
              <TableRow><TableCell colSpan={3} className="text-center text-muted-foreground py-8">
                No {config.labels.plural.toLowerCase()} yet. Click <strong>Create {config.labels.single.toLowerCase()}</strong> to get started.
              </TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <CreatePartyDialog
        config={config}
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={load}
      />
    </div>
  )
}

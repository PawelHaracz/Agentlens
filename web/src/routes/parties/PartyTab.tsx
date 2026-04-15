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

export default function PartyTab() {
  const { hasPermission } = useAuth()
  const navigate = useNavigate()
  const [groups, setGroups] = useState<Party[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const canWrite = hasPermission('users:write')

  const load = () => {
    api.listGroups().then(setGroups).catch(() => {})
  }
  useEffect(load, [])

  const handleDelete = async (g: Party) => {
    if (!confirm(`Delete group "${g.name}"?`)) return
    try {
      await api.deleteGroup(g.id)
      load()
    } catch { /* ignore */ }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Groups</h3>
        {canWrite && (
          <Button size="sm" className="gap-1" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" /> Create group
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
            {groups.map(g => (
              <TableRow
                key={g.id}
                className="cursor-pointer hover:bg-muted/50"
                onClick={() => navigate(`/settings/groups/${g.id}`)}
              >
                <TableCell className="font-medium">{g.name}</TableCell>
                <TableCell>{new Date(g.created_at).toLocaleDateString()}</TableCell>
                {canWrite && (
                  <TableCell onClick={e => e.stopPropagation()}>
                    <Button variant="ghost" size="icon" onClick={() => handleDelete(g)} title="Delete">
                      <Trash2 className="h-3.5 w-3.5 text-destructive" />
                    </Button>
                  </TableCell>
                )}
              </TableRow>
            ))}
            {groups.length === 0 && (
              <TableRow><TableCell colSpan={3} className="text-center text-muted-foreground py-8">
                No groups yet. Click <strong>Create group</strong> to get started.
              </TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <CreatePartyDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={load}
      />
    </div>
  )
}

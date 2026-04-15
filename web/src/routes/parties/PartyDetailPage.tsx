import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ChevronLeft, Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { useAuth } from '../../contexts/AuthContext'
import * as api from '@/api'
import type { Party, PartyRelationship } from '@/api'
import AddMemberDialog from './AddMemberDialog'

export default function PartyDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { hasPermission } = useAuth()
  const canWrite = hasPermission('users:write')
  const [group, setGroup] = useState<Party | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [members, setMembers] = useState<PartyRelationship[]>([])
  const [parties, setParties] = useState<Party[]>([])
  const [addOpen, setAddOpen] = useState(false)

  const partyById = useMemo(() => {
    const m = new Map<string, Party>()
    for (const p of parties) m.set(p.id, p)
    return m
  }, [parties])

  const reloadMembers = () => {
    if (!id) return
    api.listGroupMembers(id).then(setMembers).catch(() => {})
  }

  useEffect(() => {
    if (!id) return
    setNotFound(false)
    api.getGroup(id)
      .then(setGroup)
      .catch(() => setNotFound(true))
    api.listGroupMembers(id).then(setMembers).catch(() => {})
    api.listParties().then(setParties).catch(() => {})
  }, [id])

  const handleRemove = async (partyId: string, name: string) => {
    if (!id) return
    if (!confirm(`Remove "${name}" from "${group?.name ?? ''}"?`)) return
    try {
      await api.removeGroupMember(id, partyId)
      reloadMembers()
    } catch { /* ignore */ }
  }

  const excludedIds = useMemo(() => {
    const set = new Set<string>([id ?? ''])
    return set
  }, [id])

  const existingMemberIds = useMemo(() => {
    const set = new Set<string>()
    for (const r of members) set.add(r.from_party_id)
    return set
  }, [members])

  if (notFound) {
    return (
      <div className="space-y-4">
        <Link to="/settings?tab=groups" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ChevronLeft className="h-4 w-4" /> Back to Settings
        </Link>
        <p className="text-muted-foreground">Group not found.</p>
      </div>
    )
  }

  if (!group) {
    return <div className="text-muted-foreground">Loading…</div>
  }

  const memberParties = members.map(r => partyById.get(r.from_party_id)).filter(Boolean) as Party[]

  return (
    <div className="space-y-6">
      <Link to="/settings?tab=groups" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="h-4 w-4" /> Back to Settings
      </Link>
      <div>
        <h2 className="text-2xl font-bold tracking-tight">{group.name}</h2>
        <p className="text-muted-foreground">
          Created {new Date(group.created_at).toLocaleDateString()}
        </p>
      </div>
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-medium">Members</h3>
          {canWrite && (
            <Button size="sm" className="gap-1" onClick={() => setAddOpen(true)}>
              <Plus className="h-4 w-4" /> Add member
            </Button>
          )}
        </div>
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Kind</TableHead>
                {canWrite && <TableHead className="w-24">Actions</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {memberParties.map(p => (
                <TableRow key={p.id}>
                  <TableCell className="font-medium">{p.name}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">{p.kind === 'person' ? 'Person' : p.kind === 'group' ? 'Group' : 'Project'}</Badge>
                  </TableCell>
                  {canWrite && (
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => handleRemove(p.id, p.name)} title="Remove">
                        <Trash2 className="h-3.5 w-3.5 text-destructive" />
                      </Button>
                    </TableCell>
                  )}
                </TableRow>
              ))}
              {memberParties.length === 0 && (
                <TableRow><TableCell colSpan={3} className="text-center text-muted-foreground py-8">
                  No members yet. Click <strong>Add member</strong> to add the first person or group.
                </TableCell></TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </div>
      <AddMemberDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        onAdded={reloadMembers}
        groupId={id ?? ''}
        parties={parties}
        excludedIds={excludedIds}
        existingMemberIds={existingMemberIds}
      />
    </div>
  )
}

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
import EditMemberRoleDialog from './EditMemberRoleDialog'
import ProjectEntriesPanel from './ProjectEntriesPanel'
import { groupUIConfig, type PartyUIConfig } from './partyUIConfig'

interface Props {
  config?: PartyUIConfig
}

export default function PartyDetailPage({ config = groupUIConfig }: Props) {
  const { id } = useParams<{ id: string }>()
  const { hasPermission } = useAuth()
  const canWrite = hasPermission(config.writePermission)
  const [party, setParty] = useState<Party | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [members, setMembers] = useState<PartyRelationship[]>([])
  const [parties, setParties] = useState<Party[]>([])
  const [addOpen, setAddOpen] = useState(false)
  const [editRoleState, setEditRoleState] = useState<{ memberId: string; memberName: string; currentRole: string } | null>(null)

  const getFn = config.kind === 'group' ? api.getGroup : api.getProject
  const listMembersFn = config.kind === 'group' ? api.listGroupMembers : api.listProjectMembers
  const removeFn = config.kind === 'group' ? api.removeGroupMember : api.removeProjectMember

  const partyById = useMemo(() => {
    const m = new Map<string, Party>()
    for (const p of parties) m.set(p.id, p)
    return m
  }, [parties])

  const reloadMembers = () => {
    if (!id) return
    listMembersFn(id).then(setMembers).catch(() => {})
  }

  useEffect(() => {
    if (!id) return
    setNotFound(false)
    getFn(id).then(setParty).catch(() => setNotFound(true))
    listMembersFn(id).then(setMembers).catch(() => {})
    api.listParties().then(setParties).catch(() => {})
  }, [id, getFn, listMembersFn])

  const handleRemove = async (partyId: string, name: string) => {
    if (!id) return
    if (!confirm(`Remove "${name}" from "${party?.name ?? ''}"?`)) return
    try { await removeFn(id, partyId); reloadMembers() } catch { /* ignore */ }
  }

  const excludedIds = useMemo(() => new Set<string>([id ?? '']), [id])
  const existingMemberIds = useMemo(() => {
    const set = new Set<string>()
    for (const r of members) set.add(r.from_party_id)
    return set
  }, [members])

  if (notFound) {
    return (
      <div className="space-y-4">
        <Link to={`/settings?tab=${config.urlPrefix}`} className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ChevronLeft className="h-4 w-4" /> Back to Settings
        </Link>
        <p className="text-muted-foreground">{config.labels.single} not found.</p>
      </div>
    )
  }

  if (!party) return <div className="text-muted-foreground">Loading…</div>

  const memberRows = members
    .map(r => {
      const p = partyById.get(r.from_party_id)
      return p ? { party: p, rel: r } : null
    })
    .filter(Boolean) as { party: Party; rel: PartyRelationship }[]

  const colSpan = (config.showMemberRoleColumn ? 3 : 2) + (canWrite ? 1 : 0)

  return (
    <div className="space-y-6">
      <Link to={`/settings?tab=${config.urlPrefix}`} className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="h-4 w-4" /> Back to Settings
      </Link>
      <div>
        <h2 className="text-2xl font-bold tracking-tight">{party.name}</h2>
        <p className="text-muted-foreground">Created {new Date(party.created_at).toLocaleDateString()}</p>
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
                {config.showMemberRoleColumn && <TableHead>Role</TableHead>}
                {canWrite && <TableHead className="w-24">Actions</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {memberRows.map(({ party: p, rel }) => (
                <TableRow key={p.id}>
                  <TableCell className="font-medium">{p.name}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">{p.kind === 'person' ? 'Person' : p.kind === 'group' ? 'Group' : 'Project'}</Badge>
                  </TableCell>
                  {config.showMemberRoleColumn && (
                    <TableCell>
                      {canWrite ? (
                        <button
                          type="button"
                          className="inline-flex"
                          onClick={() => setEditRoleState({ memberId: p.id, memberName: p.name, currentRole: rel.from_role })}
                        >
                          <Badge variant="outline">{rel.from_role}</Badge>
                        </button>
                      ) : (
                        <Badge variant="outline">{rel.from_role}</Badge>
                      )}
                    </TableCell>
                  )}
                  {canWrite && (
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => handleRemove(p.id, p.name)} title="Remove">
                        <Trash2 className="h-3.5 w-3.5 text-destructive" />
                      </Button>
                    </TableCell>
                  )}
                </TableRow>
              ))}
              {memberRows.length === 0 && (
                <TableRow>
                  <TableCell colSpan={colSpan} className="text-center text-muted-foreground py-8">
                    No members yet. Click <strong>Add member</strong> to add the first person or group.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </div>
      {config.showEntriesPanel && id && <ProjectEntriesPanel projectId={id} canWrite={canWrite} />}
      <AddMemberDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        onAdded={reloadMembers}
        groupId={id ?? ''}
        parties={parties}
        excludedIds={excludedIds}
        existingMemberIds={existingMemberIds}
        kind={config.kind}
        roleOptions={config.memberRoleOptions}
        defaultRole={config.defaultMemberRole}
        cycleErrorMessage={config.cycleErrorMessage}
      />
      {editRoleState && id && (
        <EditMemberRoleDialog
          open={true}
          onOpenChange={(o) => { if (!o) setEditRoleState(null) }}
          onSaved={reloadMembers}
          projectId={id}
          memberPartyId={editRoleState.memberId}
          memberName={editRoleState.memberName}
          currentRole={editRoleState.currentRole}
          roleOptions={config.memberRoleOptions}
        />
      )}
    </div>
  )
}

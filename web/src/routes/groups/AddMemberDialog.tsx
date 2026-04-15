import { useState } from 'react'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import * as api from '@/api'
import type { Party } from '@/api'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAdded: () => void
  groupId: string
  parties: Party[]
  excludedIds: Set<string>
  existingMemberIds: Set<string>
}

function translateError(msg: string): string {
  if (msg.toLowerCase().includes('cycle')) {
    return "This member is already in the group's ancestry — adding them would create a cycle."
  }
  return msg
}

export default function AddMemberDialog({
  open, onOpenChange, onAdded, groupId, parties, excludedIds, existingMemberIds,
}: Props) {
  const [selectedId, setSelectedId] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const selectable = parties.filter(
    p => !excludedIds.has(p.id) && !existingMemberIds.has(p.id) && p.kind !== 'project'
  )

  const reset = () => { setSelectedId(''); setError(''); setSaving(false) }

  const handleSubmit = async () => {
    if (!selectedId) return
    setError('')
    setSaving(true)
    try {
      await api.addGroupMember(groupId, selectedId)
      onAdded()
      reset()
      onOpenChange(false)
    } catch (err) {
      setError(translateError(err instanceof Error ? err.message : 'Failed to add member'))
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) reset(); onOpenChange(o) }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add member</DialogTitle>
          <DialogDescription>Pick a person or another group to add as a member.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}
          <div className="space-y-2">
            <label htmlFor="member-select" className="text-sm font-medium">Member</label>
            <select
              id="member-select"
              role="combobox"
              aria-label="Member"
              value={selectedId}
              onChange={e => setSelectedId(e.target.value)}
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
            >
              <option value="" disabled>Select a party…</option>
              {selectable.map(p => (
                <option key={p.id} value={p.id}>
                  {p.name} ({p.kind === 'person' ? 'Person' : 'Group'})
                </option>
              ))}
            </select>
          </div>
          {selectedId && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              Selected: <Badge variant="secondary">
                {selectable.find(p => p.id === selectedId)?.kind === 'person' ? 'Person' : 'Group'}
              </Badge>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button onClick={handleSubmit} disabled={saving || !selectedId}>
            {saving ? 'Adding…' : 'Add'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

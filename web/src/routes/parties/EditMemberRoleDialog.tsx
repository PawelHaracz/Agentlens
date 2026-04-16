import { useState } from 'react'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import * as api from '@/api'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
  projectId: string
  memberPartyId: string
  memberName: string
  currentRole: string
  roleOptions: string[]
}

export default function EditMemberRoleDialog({
  open, onOpenChange, onSaved, projectId, memberPartyId, memberName, currentRole, roleOptions,
}: Props) {
  const [role, setRole] = useState(currentRole)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const reset = () => { setRole(currentRole); setError(''); setSaving(false) }

  const handleSubmit = async () => {
    setError('')
    setSaving(true)
    try {
      await api.updateProjectMemberRole(projectId, memberPartyId, role)
      onSaved()
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update role')
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) reset(); onOpenChange(o) }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit member access</DialogTitle>
          <DialogDescription>Update {memberName}'s access level on this project.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}
          <div className="space-y-2">
            <label htmlFor="edit-role-select" className="text-sm font-medium">Role</label>
            <select
              id="edit-role-select"
              value={role}
              onChange={e => setRole(e.target.value)}
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
            >
              {roleOptions.map(r => <option key={r} value={r}>{r}</option>)}
            </select>
          </div>
        </div>
        <DialogFooter>
          <Button onClick={handleSubmit} disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

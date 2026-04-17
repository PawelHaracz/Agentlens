import { useState, type FormEvent } from 'react'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import * as api from '@/api'
import { groupUIConfig, type PartyUIConfig } from './partyUIConfig'

interface Props {
  config?: PartyUIConfig
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => void
}

export default function CreatePartyDialog({ config = groupUIConfig, open, onOpenChange, onCreated }: Props) {
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const createFn = config.kind === 'group' ? api.createGroup : api.createProject
  const descriptionText = config.kind === 'group'
    ? 'Groups organize people and nested groups. Add members after creation.'
    : 'Projects scope catalog entries and group access by role. Add members after creation.'

  const reset = () => { setName(''); setError(''); setSaving(false) }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setSaving(true)
    try {
      await createFn({ name: name.trim() })
      onCreated()
      reset()
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to create ${config.labels.single.toLowerCase()}`)
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) reset(); onOpenChange(o) }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create {config.labels.single.toLowerCase()}</DialogTitle>
          <DialogDescription>{descriptionText}</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}
          <div className="space-y-2">
            <label htmlFor="party-name" className="text-sm font-medium">Name</label>
            <Input id="party-name" value={name} onChange={e => setName(e.target.value)} maxLength={128} required autoFocus />
          </div>
          <DialogFooter>
            <Button type="submit" disabled={saving || name.trim().length === 0}>
              {saving ? 'Creating…' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

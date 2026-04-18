import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuth } from '../contexts/AuthContext'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Plus, Trash2, RefreshCw, Copy, Eye, EyeOff } from 'lucide-react'
import * as api from '@/api'

export default function ServiceAccountsPage() {
  const { hasPermission } = useAuth()
  const qc = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [name, setName] = useState('')
  const [oneTimeSecret, setOneTimeSecret] = useState<string | null>(null)
  const [secretVisible, setSecretVisible] = useState(false)
  const [error, setError] = useState('')

  const { data: accounts = [], isLoading } = useQuery({
    queryKey: ['service-accounts'],
    queryFn: api.listServiceAccounts,
  })

  const createMut = useMutation({
    mutationFn: (name: string) => api.createServiceAccount(name),
    onSuccess: (res) => {
      setOneTimeSecret(res.secret)
      setCreateOpen(false)
      setName('')
      qc.invalidateQueries({ queryKey: ['service-accounts'] })
    },
    onError: () => setError('Failed to create service account'),
  })

  const deleteMut = useMutation({
    mutationFn: (id: string) => api.deleteServiceAccount(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['service-accounts'] }),
  })

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (!name.trim()) return
    createMut.mutate(name.trim())
  }

  return (
    <div className="space-y-4 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Service Accounts</h1>
          <p className="text-sm text-muted-foreground">Machine identities for API key authentication</p>
        </div>
        {hasPermission('service_accounts:write') && (
          <Button data-testid="create-sa-btn" onClick={() => setCreateOpen(true)}>
            <Plus className="mr-2 h-4 w-4" /> New Service Account
          </Button>
        )}
      </div>

      {oneTimeSecret && (
        <Card className="border-yellow-500 bg-yellow-50 dark:bg-yellow-950">
          <CardHeader>
            <CardTitle className="text-yellow-800 dark:text-yellow-200">One-Time Secret</CardTitle>
            <CardDescription>Copy this now — it will never be shown again.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="flex items-center gap-2" data-testid="one-time-secret-display">
              <code className="flex-1 rounded bg-yellow-100 dark:bg-yellow-900 px-2 py-1 text-sm break-all">
                {secretVisible ? oneTimeSecret : '•'.repeat(Math.min(oneTimeSecret.length, 40))}
              </code>
              <Button variant="ghost" size="icon" onClick={() => setSecretVisible(v => !v)}>
                {secretVisible ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </Button>
              <Button variant="ghost" size="icon" onClick={() => navigator.clipboard.writeText(oneTimeSecret)}>
                <Copy className="h-4 w-4" />
              </Button>
            </div>
            <Button variant="outline" size="sm" onClick={() => setOneTimeSecret(null)}>Dismiss</Button>
          </CardContent>
        </Card>
      )}

      <div className="rounded-md border">
        <Table data-testid="sa-table">
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>ID</TableHead>
              <TableHead>Kind</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow><TableCell colSpan={4} className="text-center">Loading…</TableCell></TableRow>
            )}
            {!isLoading && accounts.length === 0 && (
              <TableRow><TableCell colSpan={4} className="text-center text-muted-foreground">No service accounts yet</TableCell></TableRow>
            )}
            {accounts.map(sa => (
              <TableRow key={sa.id} data-testid={`sa-row-${sa.id}`}>
                <TableCell className="font-medium">{sa.name}</TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">{sa.id.slice(0, 8)}…</TableCell>
                <TableCell><Badge variant="secondary">service_account</Badge></TableCell>
                <TableCell className="text-right">
                  <div className="flex justify-end gap-1">
                    <Button variant="ghost" size="icon" title="Rotate secret"
                      onClick={() => rotateSecret(sa.id, qc, setOneTimeSecret)}>
                      <RefreshCw className="h-4 w-4" />
                    </Button>
                    {hasPermission('service_accounts:revoke') && (
                      <Button variant="ghost" size="icon" title="Delete"
                        onClick={() => deleteMut.mutate(sa.id)}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent data-testid="create-sa-dialog">
          <DialogHeader><DialogTitle>New Service Account</DialogTitle></DialogHeader>
          <form onSubmit={handleCreate} className="space-y-4">
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div className="space-y-2">
              <label className="text-sm font-medium">Name</label>
              <Input data-testid="sa-name-input" value={name}
                onChange={e => setName(e.target.value)} placeholder="my-service-account" required />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={createMut.isPending}>Create</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}

async function rotateSecret(id: string, qc: ReturnType<typeof useQueryClient>, setSecret: (s: string) => void) {
  try {
    const res = await api.rotateServiceAccountSecret(id)
    setSecret(res.secret)
    qc.invalidateQueries({ queryKey: ['service-accounts'] })
  } catch { /* handled by toast in production */ }
}

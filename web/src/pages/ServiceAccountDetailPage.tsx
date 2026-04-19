import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuth } from '../contexts/AuthContext'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ArrowLeft, RefreshCw, Trash2, Copy, Eye, EyeOff, AlertTriangle } from 'lucide-react'
import * as api from '@/api'

export default function ServiceAccountDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { hasPermission } = useAuth()
  const qc = useQueryClient()

  const [rotateConfirmOpen, setRotateConfirmOpen] = useState(false)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [oneTimeSecret, setOneTimeSecret] = useState<string | null>(null)
  const [secretVisible, setSecretVisible] = useState(false)

  const { data: accounts, isLoading } = useQuery({
    queryKey: ['service-accounts'],
    queryFn: api.listServiceAccounts,
  })
  const account = accounts?.find(a => a.id === id)

  const rotateMut = useMutation({
    mutationFn: () => api.rotateServiceAccountSecret(id),
    onSuccess: (res) => {
      setOneTimeSecret(res.secret)
      setRotateConfirmOpen(false)
      qc.invalidateQueries({ queryKey: ['service-accounts'] })
    },
  })

  const deleteMut = useMutation({
    mutationFn: () => api.deleteServiceAccount(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['service-accounts'] })
      navigate('/admin/service-accounts')
    },
  })

  if (isLoading) {
    return <div className="p-6">Loading…</div>
  }
  if (!account) {
    return (
      <div className="p-6 space-y-4">
        <Link to="/admin/service-accounts" className="flex items-center text-sm text-muted-foreground">
          <ArrowLeft className="mr-1 h-4 w-4" /> Back to service accounts
        </Link>
        <Card>
          <CardContent className="pt-6">
            <p>Service account not found.</p>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-4 p-6" data-testid="sa-detail-page">
      <Link to="/admin/service-accounts" className="flex items-center text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="mr-1 h-4 w-4" /> Back to service accounts
      </Link>

      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{account.name}</h1>
          <p className="text-sm text-muted-foreground font-mono">{account.id}</p>
        </div>
        <Badge variant="secondary">service_account</Badge>
      </div>

      {oneTimeSecret && (
        <Card className="border-yellow-500 bg-yellow-50 dark:bg-yellow-950">
          <CardHeader>
            <CardTitle className="text-yellow-800 dark:text-yellow-200">One-Time Secret</CardTitle>
            <CardDescription>Copy this now — it will never be shown again.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="flex items-center gap-2" data-testid="detail-one-time-secret">
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

      <Card>
        <CardHeader>
          <CardTitle>Credential management</CardTitle>
          <CardDescription>Rotate the API secret or delete this service account.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          {hasPermission('service_accounts:write') && (
            <Button
              variant="outline"
              data-testid="rotate-secret-btn"
              onClick={() => setRotateConfirmOpen(true)}
            >
              <RefreshCw className="mr-2 h-4 w-4" /> Rotate secret
            </Button>
          )}
          {hasPermission('service_accounts:revoke') && (
            <Button
              variant="destructive"
              data-testid="delete-sa-btn"
              onClick={() => setDeleteConfirmOpen(true)}
            >
              <Trash2 className="mr-2 h-4 w-4" /> Delete service account
            </Button>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Metadata</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1 text-sm">
          <div><span className="font-medium">Created:</span> {account.created_at || '—'}</div>
          <div><span className="font-medium">Updated:</span> {account.updated_at || '—'}</div>
        </CardContent>
      </Card>

      <Dialog open={rotateConfirmOpen} onOpenChange={setRotateConfirmOpen}>
        <DialogContent data-testid="rotate-confirm-dialog">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-yellow-600" /> Rotate secret?
            </DialogTitle>
          </DialogHeader>
          <p className="text-sm">
            The current API key will be revoked immediately. A new one-time secret will be shown once.
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRotateConfirmOpen(false)}>Cancel</Button>
            <Button
              data-testid="rotate-confirm"
              onClick={() => rotateMut.mutate()}
              disabled={rotateMut.isPending}
            >
              Rotate
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
        <DialogContent data-testid="delete-confirm-dialog">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-destructive" /> Delete service account?
            </DialogTitle>
          </DialogHeader>
          <p className="text-sm">
            All API keys for this account will be invalidated before the account is permanently removed.
            This action cannot be undone.
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteConfirmOpen(false)}>Cancel</Button>
            <Button
              variant="destructive"
              data-testid="delete-confirm"
              onClick={() => deleteMut.mutate()}
              disabled={deleteMut.isPending}
            >
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

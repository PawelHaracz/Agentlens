import { useState, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { getEntry, deleteEntry, patchLifecycle, postProbe } from '../api'
import type { CatalogEntry } from '../types'
import { useAuth } from '../contexts/AuthContext'
import StatusBadge from './StatusBadge'
import ProtocolBadge from './ProtocolBadge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { ArrowLeft, Trash2, RefreshCw, Archive, AlertCircle } from 'lucide-react'
import { cn } from '@/lib/utils'

export default function EntryDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { hasPermission } = useAuth()
  const canEdit = hasPermission('catalog:write')
  const [entry, setEntry] = useState<CatalogEntry | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [probing, setProbing] = useState(false)
  const [lifecycleLoading, setLifecycleLoading] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    setLoading(true)
    getEntry(id)
      .then(setEntry)
      .catch(e => setError(e instanceof Error ? e.message : 'Unknown error'))
      .finally(() => setLoading(false))
  }, [id])

  const handleDelete = async () => {
    if (!entry || !confirm(`Delete "${entry.display_name}"?`)) return
    setDeleting(true)
    try {
      await deleteEntry(entry.id)
      navigate('/')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
      setDeleting(false)
    }
  }

  function relativeTime(isoStr: string): string {
    const diff = Math.floor((Date.now() - new Date(isoStr).getTime()) / 1000)
    if (diff < 60) return `${diff}s ago`
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
    return `${Math.floor(diff / 3600)}h ago`
  }

  const handleProbeNow = async () => {
    if (!entry) return
    setProbing(true)
    setActionError(null)
    try {
      const health = await postProbe(entry.id)
      setEntry(prev => prev ? { ...prev, health, status: health.state } : prev)
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'Probe failed')
    } finally {
      setProbing(false)
    }
  }

  const handleDeprecate = async () => {
    if (!entry) return
    setLifecycleLoading(true)
    setActionError(null)
    try {
      const updated = await patchLifecycle(entry.id, 'deprecated')
      setEntry(updated)
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'Failed to deprecate')
    } finally {
      setLifecycleLoading(false)
    }
  }

  const handleUndeprecate = async () => {
    if (!entry) return
    setLifecycleLoading(true)
    setActionError(null)
    try {
      const updated = await patchLifecycle(entry.id, 'active')
      setEntry(updated)
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'Failed to un-deprecate')
    } finally {
      setLifecycleLoading(false)
    }
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (error) {
    return (
      <Card className="border-destructive bg-destructive/10 p-4 text-destructive text-sm">
        {error}
      </Card>
    )
  }

  if (!entry) return null

  return (
    <div>
      <div className="mb-4">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to catalog
          </Link>
        </Button>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-4">
            <div>
              <CardTitle className="text-2xl">{entry.display_name}</CardTitle>
              {entry.description && (
                <p className="text-muted-foreground mt-1">{entry.description}</p>
              )}
            </div>
            <Button
              variant="destructive"
              size="sm"
              onClick={handleDelete}
              disabled={deleting}
            >
              <Trash2 className="mr-2 h-4 w-4" />
              {deleting ? 'Deleting…' : 'Delete'}
            </Button>
          </div>

          <div className="flex flex-wrap gap-2 mt-4">
            <ProtocolBadge protocol={entry.protocol} />
            <StatusBadge status={entry.status} />
            <Badge variant="secondary">{entry.source}</Badge>
            {entry.version && (
              <Badge variant="secondary">v{entry.version}</Badge>
            )}
            {entry.spec_version && (
              <Badge variant="outline">Spec v{entry.spec_version}</Badge>
            )}
          </div>
        </CardHeader>

        <CardContent>
          <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
            <Field label="Endpoint" value={entry.endpoint} mono />
            {entry.metadata?.['kubernetes.namespace'] && (
              <Field label="Namespace" value={entry.metadata['kubernetes.namespace']} />
            )}
            {entry.provider?.team && <Field label="Team" value={entry.provider.team} />}
            {entry.provider?.organization && <Field label="Organization" value={entry.provider.organization} />}
            <Field label="Last Seen" value={new Date(entry.validity.last_seen).toLocaleString()} />
            <Field label="Created" value={new Date(entry.created_at).toLocaleString()} />
          </dl>

          {entry.categories && entry.categories.length > 0 && (
            <>
              <Separator className="my-4" />
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Categories</p>
                <div className="flex flex-wrap gap-1">
                  {entry.categories.map(cat => (
                    <Badge key={cat} variant="outline" className="bg-primary/5">
                      {cat}
                    </Badge>
                  ))}
                </div>
              </div>
            </>
          )}

          {entry.capabilities && entry.capabilities.length > 0 && (
            <>
              <Separator className="my-4" />
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide mb-3">
                  Capabilities ({entry.capabilities.length})
                </p>
                <div className="space-y-2">
                  {entry.capabilities.map((cap, i) => (
                    <Card key={i} className="bg-muted/50">
                      <CardContent className="p-3">
                        <div className="flex items-center gap-2 mb-0.5">
                          <p className="font-medium text-sm">{cap.name}</p>
                          <Badge variant="outline" className="text-xs">{cap.kind}</Badge>
                        </div>
                        {cap.description && (
                          <p className="text-xs text-muted-foreground mt-0.5">{String(cap.description)}</p>
                        )}
                      </CardContent>
                    </Card>
                  ))}
                </div>
              </div>
            </>
          )}

          {entry.raw_definition != null && (
            <>
              <Separator className="my-4" />
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Raw Definition</p>
                <ScrollArea className="h-64 rounded-md border">
                  <pre className="p-4 text-xs font-mono">
                    {JSON.stringify(entry.raw_definition, null, 2)}
                  </pre>
                </ScrollArea>
              </div>
            </>
          )}

          <Separator className="my-4" />
          <div>
            <div className="flex items-center justify-between mb-3">
              <h3 className="font-semibold text-sm">Health</h3>
              {canEdit && (
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={probing || entry.status === 'deprecated'}
                    onClick={handleProbeNow}
                  >
                    <RefreshCw className={cn('mr-2 h-4 w-4', probing && 'animate-spin')} />
                    Probe now
                  </Button>

                  {entry.status === 'deprecated' ? (
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={lifecycleLoading}
                      onClick={handleUndeprecate}
                    >
                      <Archive className="mr-2 h-4 w-4" />
                      Un-deprecate
                    </Button>
                  ) : (
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="outline" size="sm" disabled={lifecycleLoading}>
                          <Archive className="mr-2 h-4 w-4" />
                          Deprecate
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Deprecate this entry?</AlertDialogTitle>
                          <AlertDialogDescription>
                            The health prober will stop monitoring this entry. You can un-deprecate it later.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction onClick={handleDeprecate}>Deprecate</AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  )}
                </div>
              )}
            </div>

            {actionError && (
              <Alert variant="destructive" className="mb-3">
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>{actionError}</AlertDescription>
              </Alert>
            )}

            <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
              <dt className="text-muted-foreground">State</dt>
              <dd><StatusBadge status={entry.status} /></dd>

              <dt className="text-muted-foreground">Last probed</dt>
              <dd>
                {entry.health?.lastProbedAt
                  ? <span title={new Date(entry.health.lastProbedAt).toUTCString()}>
                      {relativeTime(entry.health.lastProbedAt)}
                    </span>
                  : <span className="text-muted-foreground">—</span>}
              </dd>

              <dt className="text-muted-foreground">Last successful</dt>
              <dd>
                {entry.health?.lastSuccessAt
                  ? <span title={new Date(entry.health.lastSuccessAt).toUTCString()}>
                      {relativeTime(entry.health.lastSuccessAt)}
                    </span>
                  : <span className="text-muted-foreground">—</span>}
              </dd>

              <dt className="text-muted-foreground">Latency</dt>
              <dd>
                {(entry.health?.latencyMs ?? 0) > 0
                  ? `${entry.health.latencyMs} ms`
                  : <span className="text-muted-foreground">—</span>}
              </dd>

              <dt className="text-muted-foreground">Failures (run)</dt>
              <dd>{entry.health?.consecutiveFailures ?? 0}</dd>

              <dt className="text-muted-foreground">Last error</dt>
              <dd className="font-mono text-xs break-all">
                {entry.health?.lastError || <span className="text-muted-foreground">—</span>}
              </dd>
            </dl>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-xs text-muted-foreground uppercase tracking-wide">{label}</dt>
      <dd className={`mt-0.5 text-foreground break-all ${mono ? 'font-mono text-xs' : ''}`}>{value}</dd>
    </div>
  )
}

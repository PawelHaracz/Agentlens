import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { AlertCircle, ArrowLeft, RefreshCw } from 'lucide-react'
import type { CatalogEntry, LifecycleState } from '../../types'
import { getEntry, postProbe, patchLifecycle } from '../../api'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../../components/ui/tabs'
import { Button } from '../../components/ui/button'
import { Skeleton } from '../../components/ui/skeleton'
import { Alert, AlertTitle, AlertDescription } from '../../components/ui/alert'
import StatusBadge from '../../components/StatusBadge'
import ProtocolBadge from '../../components/ProtocolBadge'
import { SpecVersionBadge } from './components/SpecVersionBadge'
import { RawCardTab } from './components/RawCardTab'

type PageStatus = 'loading' | 'success' | 'error'

export default function CatalogDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [pageStatus, setPageStatus] = useState<PageStatus>('loading')
  const [entry, setEntry] = useState<CatalogEntry | null>(null)
  const [fetchError, setFetchError] = useState<string>('')

  const [probing, setProbing] = useState(false)
  const [lifecycleLoading, setLifecycleLoading] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const fetchEntry = useCallback(async () => {
    if (!id) return
    setPageStatus('loading')
    setFetchError('')
    try {
      const data = await getEntry(id)
      setEntry(data)
      setPageStatus('success')
    } catch (err) {
      setFetchError(err instanceof Error ? err.message : String(err))
      setPageStatus('error')
    }
  }, [id])

  useEffect(() => {
    fetchEntry()
  }, [fetchEntry])

  const handleProbe = async () => {
    if (!id) return
    setProbing(true)
    try {
      await postProbe(id)
      await fetchEntry()
    } catch {
      // probe errors are non-critical; re-fetch to get latest state
    } finally {
      setProbing(false)
    }
  }

  const handleLifecycle = async (newState: LifecycleState) => {
    if (!id) return
    setLifecycleLoading(true)
    setActionError(null)
    try {
      const updated = await patchLifecycle(id, newState)
      setEntry(updated)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to update lifecycle state')
    } finally {
      setLifecycleLoading(false)
    }
  }

  if (pageStatus === 'loading') {
    return (
      <div className="container mx-auto py-6 space-y-4">
        <Skeleton className="h-8 w-48 rounded-md" />
        <Skeleton className="h-6 w-full rounded-md" />
        <Skeleton className="h-6 w-3/4 rounded-md" />
        <Skeleton className="h-64 w-full rounded-md" />
      </div>
    )
  }

  if (pageStatus === 'error') {
    return (
      <div className="container mx-auto py-6">
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>Failed to load entry</AlertTitle>
          <AlertDescription className="flex items-center gap-3 mt-1">
            <span>{fetchError}</span>
            <Button size="sm" variant="outline" onClick={fetchEntry} className="shrink-0">
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      </div>
    )
  }

  if (!entry) return null

  const isDeprecated = entry.status === 'deprecated'

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header */}
      <div className="space-y-3">
        <Button variant="ghost" size="sm" onClick={() => navigate(-1)} className="-ml-2">
          <ArrowLeft className="mr-1.5 h-4 w-4" />
          Back
        </Button>

        <div className="flex flex-wrap items-start gap-3">
          <h1 className="text-2xl font-semibold leading-tight">{entry.display_name}</h1>
          <div className="flex flex-wrap items-center gap-2 mt-0.5">
            <ProtocolBadge protocol={entry.protocol} />
            <StatusBadge
              status={entry.status}
              latencyMs={entry.health?.latencyMs}
              lastSeenAt={entry.health?.lastSuccessAt ?? undefined}
            />
            <SpecVersionBadge version={entry.spec_version} />
          </div>
        </div>

        {entry.description && (
          <p className="text-muted-foreground text-sm max-w-2xl">{entry.description}</p>
        )}

        {/* Action buttons */}
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleProbe}
            disabled={probing}
          >
            <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${probing ? 'animate-spin' : ''}`} />
            {probing ? 'Probing…' : 'Probe Now'}
          </Button>

          {isDeprecated ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleLifecycle('registered')}
              disabled={lifecycleLoading}
            >
              Undeprecate
            </Button>
          ) : (
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleLifecycle('deprecated')}
              disabled={lifecycleLoading}
            >
              Deprecate
            </Button>
          )}
        </div>
      </div>

      {/* Lifecycle action error */}
      {actionError && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>Action failed</AlertTitle>
          <AlertDescription>{actionError}</AlertDescription>
        </Alert>
      )}

      {/* Tabbed content */}
      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="raw">Raw Card</TabsTrigger>
        </TabsList>

        {/* Overview tab */}
        <TabsContent value="overview" className="space-y-6 pt-4">
          {/* Fields grid */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">Endpoint</p>
              <p className="text-sm font-mono break-all">{entry.endpoint}</p>
            </div>
            <div>
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">Source</p>
              <p className="text-sm">{entry.source}</p>
            </div>
            <div>
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">Version</p>
              <p className="text-sm font-mono">{entry.version || '—'}</p>
            </div>
            {entry.provider && (
              <div>
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">Provider</p>
                <p className="text-sm">
                  {entry.provider.organization}
                  {entry.provider.team && <span className="text-muted-foreground"> / {entry.provider.team}</span>}
                </p>
              </div>
            )}
          </div>

          {/* Categories */}
          {entry.categories && entry.categories.length > 0 && (
            <div>
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2">Categories</p>
              <div className="flex flex-wrap gap-2">
                {entry.categories.map((cat) => (
                  <span
                    key={cat}
                    className="inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold text-foreground"
                  >
                    {cat}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* Capabilities */}
          {entry.capabilities && entry.capabilities.length > 0 && (
            <div>
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2">
                Capabilities ({entry.capabilities.length})
              </p>
              <ul className="space-y-2">
                {entry.capabilities.map((cap, i) => (
                  <li key={`${cap.kind}-${cap.name}-${i}`} className="rounded-md border px-3 py-2">
                    <p className="text-sm font-medium">{cap.name}</p>
                    {cap.description && (
                      <p className="text-xs text-muted-foreground mt-0.5">{cap.description}</p>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Health */}
          <div>
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2">Health</p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <p className="text-xs text-muted-foreground mb-0.5">Latency</p>
                <p className="text-sm">
                  {entry.health?.latencyMs != null && entry.health.latencyMs > 0
                    ? `${entry.health.latencyMs} ms`
                    : '—'}
                </p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground mb-0.5">Consecutive Failures</p>
                <p className="text-sm">{entry.health?.consecutiveFailures ?? 0}</p>
              </div>
              {entry.health?.lastError && (
                <div className="sm:col-span-2">
                  <p className="text-xs text-muted-foreground mb-0.5">Last Error</p>
                  <p className="text-sm text-destructive font-mono break-all">{entry.health.lastError}</p>
                </div>
              )}
            </div>
          </div>
        </TabsContent>

        {/* Raw Card tab */}
        <TabsContent value="raw" className="pt-4">
          <RawCardTab entryId={entry.id} displayName={entry.display_name} />
        </TabsContent>
      </Tabs>
    </div>
  )
}

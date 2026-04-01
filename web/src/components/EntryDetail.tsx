import { useState, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { getEntry, deleteEntry } from '../api'
import type { CatalogEntry } from '../types'
import StatusBadge from './StatusBadge'
import ProtocolBadge from './ProtocolBadge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { ArrowLeft, Trash2 } from 'lucide-react'

export default function EntryDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [entry, setEntry] = useState<CatalogEntry | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)

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

          {entry.skills && entry.skills.length > 0 && (
            <>
              <Separator className="my-4" />
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide mb-3">
                  Skills ({entry.skills.length})
                </p>
                <div className="space-y-2">
                  {entry.skills.map((skill, i) => (
                    <Card key={i} className="bg-muted/50">
                      <CardContent className="p-3">
                        <p className="font-medium text-sm">{skill.name}</p>
                        {skill.description && (
                          <p className="text-xs text-muted-foreground mt-0.5">{skill.description}</p>
                        )}
                        <div className="flex gap-4 mt-2 text-xs text-muted-foreground">
                          {skill.input_modes && skill.input_modes.length > 0 && (
                            <span>In: {skill.input_modes.join(', ')}</span>
                          )}
                          {skill.output_modes && skill.output_modes.length > 0 && (
                            <span>Out: {skill.output_modes.join(', ')}</span>
                          )}
                        </div>
                      </CardContent>
                    </Card>
                  ))}
                </div>
              </div>
            </>
          )}

          {entry.typed_meta && entry.typed_meta.filter(m => m.kind === 'a2a.extension').length > 0 && (
            <>
              <Separator className="my-4" />
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">
                  Extensions ({entry.typed_meta.filter(m => m.kind === 'a2a.extension').length})
                </p>
                <div className="space-y-1">
                  {entry.typed_meta.filter(m => m.kind === 'a2a.extension').map((ext, i) => (
                    <div key={i} className="flex items-center gap-2 text-sm">
                      <span className="font-mono break-all">{(ext as unknown as { uri: string }).uri}</span>
                      <Badge variant={(ext as unknown as { required: boolean }).required ? 'destructive' : 'secondary'} className="text-xs">
                        {(ext as unknown as { required: boolean }).required ? 'Required' : 'Optional'}
                      </Badge>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}

          {entry.typed_meta && entry.typed_meta.filter(m => m.kind === 'a2a.security_scheme').length > 0 && (
            <>
              <Separator className="my-4" />
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Security</p>
                <div className="flex flex-wrap gap-1">
                  {entry.typed_meta.filter(m => m.kind === 'a2a.security_scheme').map((scheme, i) => (
                    <Badge key={i} variant="outline">
                      {(scheme as unknown as { type: string }).type}
                    </Badge>
                  ))}
                </div>
              </div>
            </>
          )}

          {entry.raw_card != null && (
            <>
              <Separator className="my-4" />
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Raw Card</p>
                <ScrollArea className="h-64 rounded-md border">
                  <pre className="p-4 text-xs font-mono">
                    {JSON.stringify(entry.raw_card, null, 2)}
                  </pre>
                </ScrollArea>
              </div>
            </>
          )}
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

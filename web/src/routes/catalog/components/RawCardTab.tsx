import { useState, useEffect, useRef } from 'react'
import Prism from 'prismjs'
import 'prismjs/components/prism-json'
import { getRawCard } from '@/api'
import { Alert, AlertDescription } from '../../../components/ui/alert'
import { Button } from '../../../components/ui/button'
import { Skeleton } from '../../../components/ui/skeleton'

interface Props {
  entryId: string
  displayName: string
}

type Status = 'loading' | 'success' | 'error'

function toKebab(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}

export function RawCardTab({ entryId, displayName }: Props) {
  const [status, setStatus] = useState<Status>('loading')
  const [pretty, setPretty] = useState('')
  const [fetchedAt, setFetchedAt] = useState('')
  const [truncated, setTruncated] = useState(false)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => () => { if (timerRef.current) clearTimeout(timerRef.current) }, [])

  useEffect(() => {
    setStatus('loading')
    setError('')
    setPretty('')

    getRawCard(entryId)
      .then(({ data, fetchedAt: fa, truncated: tr }) => {
        let formatted: string
        try {
          formatted = JSON.stringify(JSON.parse(data), null, 2)
        } catch {
          formatted = data
        }
        setPretty(formatted)
        setFetchedAt(fa)
        setTruncated(tr)
        setStatus('success')
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : String(err))
        setStatus('error')
      })
  }, [entryId])

  useEffect(() => {
    if (status === 'success') {
      Prism.highlightAll()
    }
  }, [status])

  if (status === 'loading') {
    return <Skeleton className="h-64 w-full rounded-md" />
  }

  if (status === 'error') {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  function handleCopy() {
    navigator.clipboard.writeText(pretty).then(() => {
      setCopied(true)
      if (timerRef.current) clearTimeout(timerRef.current)
      timerRef.current = setTimeout(() => setCopied(false), 2000)
    }).catch(() => {})
  }

  function handleDownload() {
    const blob = new Blob([pretty], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${toKebab(displayName)}.json`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    // Defer revocation so the browser has time to initiate the download.
    setTimeout(() => URL.revokeObjectURL(url), 100)
  }

  return (
    <div className="space-y-3">
      {truncated && (
        <Alert>
          <AlertDescription>
            The raw card was truncated because it exceeded the maximum allowed size.
          </AlertDescription>
        </Alert>
      )}

      <div className="flex items-center justify-between gap-2">
        {fetchedAt && (
          <p className="text-xs text-muted-foreground">
            Fetched at: {new Date(fetchedAt).toLocaleString()}
          </p>
        )}
        <div className="flex gap-2 ml-auto">
          <Button variant="outline" size="sm" onClick={handleCopy}>
            {copied ? 'Copied!' : 'Copy'}
          </Button>
          <Button variant="outline" size="sm" onClick={handleDownload}>
            Download
          </Button>
        </div>
      </div>

      <pre className="rounded-md bg-muted p-4 overflow-auto text-sm max-h-[600px]">
        <code className="language-json">{pretty}</code>
      </pre>
    </div>
  )
}

import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { useCallback } from 'react'
import { listCapabilities } from '../api'
import type { CapabilityListResult } from '../types'

export function useCapabilitiesQuery() {
  const [searchParams, setSearchParams] = useSearchParams()

  const query = searchParams.get('q') || ''
  const kind = searchParams.get('kind') || ''
  const sort = searchParams.get('sort') || 'name_asc'

  const result = useQuery<CapabilityListResult>({
    queryKey: ['capabilities', query, kind, sort],
    queryFn: () =>
      listCapabilities({
        q: query || undefined,
        kind: kind || undefined,
        sort,
        limit: 50,
      }),
  })

  const setQuery = useCallback(
    (value: string) => {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev)
        if (value) {
          next.set('q', value)
        } else {
          next.delete('q')
        }
        return next
      })
    },
    [setSearchParams]
  )

  const setKind = useCallback(
    (value: string) => {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev)
        if (value) {
          next.set('kind', value)
        } else {
          next.delete('kind')
        }
        return next
      })
    },
    [setSearchParams]
  )

  const setSort = useCallback(
    (value: string) => {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev)
        next.set('sort', value)
        return next
      })
    },
    [setSearchParams]
  )

  const clearFilters = useCallback(() => {
    setSearchParams({})
  }, [setSearchParams])

  return {
    ...result,
    query,
    kind,
    sort,
    setQuery,
    setKind,
    setSort,
    clearFilters,
  }
}

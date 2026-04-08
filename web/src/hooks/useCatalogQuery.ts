import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { useCallback } from 'react'
import { listCatalog } from '../api'
import type { ListFilter, Protocol } from '../types'

export function useCatalogQuery() {
  const [searchParams, setSearchParams] = useSearchParams()

  const protocol = (searchParams.get('protocol') as Protocol) || undefined
  const q = searchParams.get('q') || undefined
  const sort = (searchParams.get('sort') as ListFilter['sort']) || undefined

  const filter: ListFilter = {
    protocol,
    q,
    sort,
  }

  const result = useQuery({
    queryKey: ['catalog', { protocol, q, sort }],
    queryFn: () => listCatalog(filter),
  })

  const setProtocol = useCallback(
    (p: Protocol | undefined) => {
      setSearchParams(prev => {
        const next = new URLSearchParams(prev)
        if (p) next.set('protocol', p)
        else next.delete('protocol')
        return next
      })
    },
    [setSearchParams]
  )

  const setQuery = useCallback(
    (newQ: string) => {
      setSearchParams(prev => {
        const next = new URLSearchParams(prev)
        if (newQ) next.set('q', newQ)
        else next.delete('q')
        return next
      })
    },
    [setSearchParams]
  )

  const setSort = useCallback(
    (newSort: ListFilter['sort']) => {
      setSearchParams(prev => {
        const next = new URLSearchParams(prev)
        if (newSort) next.set('sort', newSort)
        else next.delete('sort')
        return next
      })
    },
    [setSearchParams]
  )

  const clearFilters = useCallback(() => {
    setSearchParams({})
  }, [setSearchParams])

  return {
    entries: result.data,
    isLoading: result.isLoading,
    isError: result.isError,
    error: result.error as Error | null,
    filter,
    setProtocol,
    setQuery,
    setSort,
    clearFilters,
    refetch: result.refetch,
  }
}

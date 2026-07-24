import { useMemo } from 'react'
import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { fetchList, fetchPublicList } from '@/lib/api/resources'
import type { ViewComponent } from '@/lib/api/views'
import { applyComponentFilters } from './view-filters'
import { fetchViewByRoute } from '@/lib/api/views'

/**
 * Fetches data for a view component based on its bindings and filters.
 * Uses the authenticated API path by default; switches to public path
 * when isPublic is true.
 */
export function useComponentData(
  component: ViewComponent,
  isPublic: boolean,
) {
  const doctype = component.source_doctype

  const query = useQuery({
    // Components on the same view commonly share a data source (for example
    // POS category tabs and the product grid). Cache the source once, then
    // apply each component's presentation filter to the shared result.
    queryKey: ['view-data', doctype, isPublic],
    queryFn: () => {
      if (!doctype) return { data: [], meta: { total: 0, doctype: '' } }
      // For public views, use the public resource API.
      if (isPublic) {
        return fetchPublicList(doctype, { limit: 50 })
      }
      return fetchList(doctype, { limit: 50 })
    },
    enabled: !!doctype,
    staleTime: 30_000,
    placeholderData: keepPreviousData,
  })

  const data = useMemo(
    () => applyComponentFilters(query.data, component.filters),
    [query.data, component.filters],
  )

  return { ...query, data }
}

export { applyComponentFilters } from './view-filters'

/**
 * Fetches a view config by route name.
 */
export function useViewConfig(viewName: string, version?: string) {
  return useQuery({
    queryKey: ['view', viewName, version],
    queryFn: () => fetchViewByRoute(viewName, version),
    staleTime: 10 * 60_000,
  })
}

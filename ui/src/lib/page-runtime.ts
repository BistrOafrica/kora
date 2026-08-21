import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { fetchPageManifestByRoute } from '@/lib/api/page-manifests'

export interface PageRuntimeRequest {
  route: string
  version?: string
}

export function usePageManifest({ route, version }: PageRuntimeRequest) {
  return useQuery({
    queryKey: ['page-manifest', route, version],
    queryFn: async () => fetchPageManifestByRoute(route, version),
    staleTime: 10 * 60_000,
    placeholderData: keepPreviousData,
  })
}

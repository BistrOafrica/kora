import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api/client'
import type { ViewComponentProps } from './registry'
import { Skeleton } from '@/components/ui/skeleton'
import { TrendingUp, TrendingDown, Minus } from 'lucide-react'

/**
 * MetricCard displays a single KPI metric with an optional trend indicator.
 * Fetches aggregate data from the view data endpoint.
 */
export default function MetricCard(props: ViewComponentProps) {
  const { config } = props

  const { data, isLoading } = useQuery({
    queryKey: ['metric', config.id, config.bindings],
    queryFn: async () => {
      const result = await api.get<any>(`/api/v1/view/data?view=${encodeURIComponent(config.bindings?.view || '')}&component=${encodeURIComponent(config.id)}`)
      return result
    },
    enabled: !!config.bindings?.view,
    staleTime: 60_000,
  })

  if (isLoading) {
    return (
      <div className="rounded-lg border p-4 space-y-2">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-8 w-16" />
      </div>
    )
  }

  const count = data?.count ?? 0
  const label = config.label || config.bindings?.title || 'Metric'
  const trend = config.bindings?.trend

  return (
    <div className="rounded-lg border bg-card p-4 hover:shadow-sm transition-shadow">
      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">{label}</p>
      <div className="mt-2 flex items-baseline gap-2">
        <span className="text-3xl font-bold">{count.toLocaleString()}</span>
        {trend === 'up' && <TrendingUp className="h-5 w-5 text-green-500" />}
        {trend === 'down' && <TrendingDown className="h-5 w-5 text-red-500" />}
        {trend === 'flat' && <Minus className="h-5 w-5 text-muted-foreground" />}
      </div>
    </div>
  )
}

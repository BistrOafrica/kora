import type { ViewComponentProps } from './registry'
import { Skeleton } from '@/components/ui/skeleton'
import { TrendingUp, TrendingDown, Minus } from 'lucide-react'

/**
 * MetricCard displays a single KPI metric with an optional trend indicator.
 * Uses resource data supplied by the PageManifest renderer.
 */
export default function MetricCard(props: ViewComponentProps) {
  const { config, data: manifestData, isLoading: manifestLoading } = props

  if (manifestLoading) {
    return (
      <div className="rounded-lg border p-4 space-y-2">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-8 w-16" />
      </div>
    )
  }

  if (!manifestData) {
    return (
      <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4" role="status">
        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{config.label || 'Metric'}</p>
        <p className="mt-2 text-sm font-medium text-destructive">Metric needs a data source</p>
        <p className="mt-1 text-xs text-muted-foreground">Bind this card to a PageManifest resource.</p>
      </div>
    )
  }

  const rows = Array.isArray(manifestData?.data) ? manifestData.data : []
  const count = summarizeRows(rows, config.bindings)
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

function summarizeRows(rows: Array<Record<string, unknown>>, bindings?: Record<string, string>): number {
  const filtered = bindings?.filter_field
    ? rows.filter((row) => String(row[bindings.filter_field!] ?? '') === String(bindings.filter_value ?? ''))
    : rows

  if (bindings?.metric === 'sum' && bindings.value_field) {
    return filtered.reduce((total, row) => total + Number(row[bindings.value_field!] ?? 0), 0)
  }

  return filtered.length
}

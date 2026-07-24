import { useQuery } from '@tanstack/react-query'
import { fetchList } from '@/lib/api/resources'
import type { ViewComponentProps } from './registry'
import { Skeleton } from '@/components/ui/skeleton'
import { format } from 'date-fns'

/**
 * RecentRecords shows the last N records of a doctype as a compact list.
 * Used on dashboards and workspace sidebars.
 */
export default function RecentRecords(props: ViewComponentProps) {
  const { config, onAction } = props
  const doctype = config.source_doctype
  const limit = parseInt(config.bindings?.limit || '5')
  const titleField = config.bindings?.title || 'name'
  const dateField = config.bindings?.date || 'modified'

  const { data, isLoading } = useQuery({
    queryKey: ['recent-records', doctype, limit],
    queryFn: () => fetchList(doctype || '', { limit, order_by: `${dateField} DESC` }),
    enabled: !!doctype,
    staleTime: 30_000,
  })

  if (isLoading) {
    return <div className="space-y-2">{Array.from({ length: limit }).map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}</div>
  }

  const records = data?.data || []

  return (
    <div className="rounded-lg border">
      <div className="px-4 py-2 border-b bg-muted/20">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{config.label || 'Recent'}</h3>
      </div>
      <div className="divide-y">
        {records.length === 0 ? (
          <p className="px-4 py-3 text-sm text-muted-foreground">No records</p>
        ) : (
          records.map((r: any) => (
            <div
              key={r.name}
              className="px-4 py-2 flex items-center justify-between hover:bg-muted/30 cursor-pointer text-sm"
              onClick={() => onAction('select', { name: r.name, row: r })}
            >
              <span className="truncate font-medium">{r[titleField] || r.name}</span>
              {r[dateField] && (
                <span className="text-xs text-muted-foreground ml-2 shrink-0">
                  {format(new Date(r[dateField]), 'MMM d')}
                </span>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  )
}

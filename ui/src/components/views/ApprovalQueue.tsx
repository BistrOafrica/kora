import type { ViewComponentProps } from './registry'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Loader2, CheckCircle, XCircle } from 'lucide-react'

/**
 * ApprovalQueue shows a list of pending items with approve/reject actions.
 * Items are filtered by the configured status binding.
 */
export default function ApprovalQueue(props: ViewComponentProps) {
  const { data, isLoading, config, onAction } = props

  if (isLoading) {
    return (
      <div className="flex items-center justify-center p-8">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  const titleField = config.bindings?.title || 'name'
  const records = data?.data || []

  return (
    <div className="space-y-2">
      {records.length === 0 ? (
        <div className="rounded-lg border p-8 text-center text-sm text-muted-foreground">
          No pending approvals
        </div>
      ) : (
        records.map((item: any) => (
          <div
            key={item.name}
            className="flex items-center justify-between rounded-lg border p-3 hover:bg-muted/30"
          >
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium truncate">{item[titleField] || item.name}</p>
              <p className="text-xs text-muted-foreground">{item.name}</p>
            </div>
            <div className="flex items-center gap-2 ml-4">
              <Badge variant="outline" className="text-xs">{item.status || 'Pending'}</Badge>
              <Button
                size="sm"
                variant="outline"
                className="text-green-600"
                onClick={() => onAction('approve', { name: item.name, action: 'Approve', row: item })}
              >
                <CheckCircle className="h-4 w-4 mr-1" />
                Approve
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="text-red-600"
                onClick={() => onAction('reject', { name: item.name, action: 'Reject', row: item })}
              >
                <XCircle className="h-4 w-4 mr-1" />
                Reject
              </Button>
            </div>
          </div>
        ))
      )}
    </div>
  )
}

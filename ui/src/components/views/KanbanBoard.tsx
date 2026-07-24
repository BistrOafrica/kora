import { useState } from 'react'
import type { ViewComponentProps } from './registry'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Loader2, GripVertical } from 'lucide-react'

interface KanbanCard {
  name: string
  [key: string]: any
}

/**
 * KanbanBoard renders workflow states as columns with draggable cards.
 * Cards are grouped by the configured status field from bindings.
 */
export default function KanbanBoard(props: ViewComponentProps) {
  const { data, isLoading, config, onAction } = props

  if (isLoading) {
    return (
      <div className="flex items-center justify-center p-8">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  const statusField = config.bindings?.status || 'status'
  const titleField = config.bindings?.title || 'name'
  const priorityField = config.bindings?.priority

  const records: KanbanCard[] = data?.data || []
  const columns = groupBy(records, statusField)
  const columnOrder = Object.keys(columns).sort()

  // Workflow states from view config or inferred from data.
  const stateStyles: Record<string, string> = {
    'Draft': 'bg-gray-100 dark:bg-gray-800',
    'Open': 'bg-blue-50 dark:bg-blue-950',
    'In Progress': 'bg-amber-50 dark:bg-amber-950',
    'Pending': 'bg-purple-50 dark:bg-purple-950',
    'Resolved': 'bg-green-50 dark:bg-green-950',
    'Closed': 'bg-green-100 dark:bg-green-900',
    'Cancelled': 'bg-red-50 dark:bg-red-950',
  }

  return (
    <div className="flex gap-3 overflow-x-auto pb-4">
      {columnOrder.map((state) => (
        <div
          key={state}
          className={`flex-shrink-0 w-72 rounded-lg border ${stateStyles[state] || 'bg-muted/20'}`}
        >
          <div className="flex items-center justify-between px-3 py-2 border-b">
            <span className="text-sm font-semibold">{state}</span>
            <Badge variant="secondary">{columns[state].length}</Badge>
          </div>
          <div className="p-2 space-y-2 min-h-[100px]">
            {columns[state].map((card) => (
              <Card
                key={card.name}
                className="cursor-pointer hover:shadow-md transition-shadow"
                onClick={() => onAction('select', { name: card.name, row: card })}
              >
                <CardHeader className="p-3 pb-1">
                  <div className="flex items-start gap-2">
                    <GripVertical className="h-4 w-4 mt-0.5 text-muted-foreground shrink-0" />
                    <div className="min-w-0">
                      <CardTitle className="text-sm truncate">
                        {card[titleField] || card.name}
                      </CardTitle>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="p-3 pt-0">
                  {priorityField && card[priorityField] && (
                    <Badge variant="outline" className="text-xs">
                      {card[priorityField]}
                    </Badge>
                  )}
                </CardContent>
              </Card>
            ))}
            {columns[state].length === 0 && (
              <p className="text-xs text-muted-foreground p-4 text-center">No items</p>
            )}
          </div>
        </div>
      ))}
    </div>
  )
}

function groupBy<T extends Record<string, any>>(items: T[], key: string): Record<string, T[]> {
  const groups: Record<string, T[]> = {}
  for (const item of items) {
    const val = item[key] || 'Unknown'
    if (!groups[val]) groups[val] = []
    groups[val].push(item)
  }
  return groups
}

import { useState } from 'react'
import type { ViewComponentProps } from './registry'
import { CheckSquare, Square } from 'lucide-react'

interface CheckItem {
  label: string
  done: boolean
}

/**
 * Checklist renders a list of tasks with completion toggles.
 */
export default function Checklist(props: ViewComponentProps) {
  const { config, onAction } = props
  const defaults: CheckItem[] = config.bindings?.items
    ? JSON.parse(config.bindings.items)
    : [{ label: 'Item 1', done: false }, { label: 'Item 2', done: false }]

  const [items, setItems] = useState<CheckItem[]>(defaults)

  const toggle = (i: number) => {
    const updated = items.map((item, j) => j === i ? { ...item, done: !item.done } : item)
    setItems(updated)
    onAction('change', { items: updated, allDone: updated.every(it => it.done) })
  }

  const done = items.filter(i => i.done).length

  return (
    <div className="rounded-lg border p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold">{config.label || 'Checklist'}</h3>
        <span className="text-xs text-muted-foreground">{done}/{items.length}</span>
      </div>
      <div className="space-y-2">
        {items.map((item, i) => (
          <button
            key={i}
            className="flex items-center gap-3 w-full text-left rounded-md p-2 hover:bg-muted/30 transition-colors"
            onClick={() => toggle(i)}
          >
            {item.done ? <CheckSquare className="h-5 w-5 text-green-500 shrink-0" /> : <Square className="h-5 w-5 text-muted-foreground shrink-0" />}
            <span className={`text-sm ${item.done ? 'line-through text-muted-foreground' : ''}`}>{item.label}</span>
          </button>
        ))}
      </div>
    </div>
  )
}

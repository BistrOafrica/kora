import { useState } from 'react'
import type { ViewComponentProps } from './registry'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Plus, Trash2 } from 'lucide-react'

/**
 * LineItemBuilder is a generic parent + child inline form.
 * Used for Sale Items, Purchase Order lines, etc.
 */
export default function LineItemBuilder(props: ViewComponentProps) {
  const { config, onAction } = props
  const [rows, setRows] = useState<Record<string, any>[]>([{}])

  const addRow = () => setRows([...rows, {}])
  const removeRow = (i: number) => setRows(rows.filter((_, j) => j !== i))
  const updateRow = (i: number, field: string, value: any) => {
    const updated = [...rows]
    updated[i] = { ...updated[i], [field]: value }
    setRows(updated)
    onAction('change', { items: updated })
  }

  const columns = config.desktop_columns || ['item', 'quantity', 'rate', 'amount']

  return (
    <div className="rounded-lg border">
      <div className="flex items-center justify-between px-4 py-2 border-b bg-muted/20">
        <h3 className="text-sm font-semibold">{config.label || 'Line Items'}</h3>
        <Button variant="outline" size="sm" onClick={addRow}><Plus className="h-3.5 w-3.5 mr-1" />Add Line</Button>
      </div>
      <div className="divide-y">
        {rows.map((row, i) => (
          <div key={i} className="flex items-center gap-2 px-4 py-2">
            {columns.map(col => (
              <Input
                key={col}
                value={row[col] ?? ''}
                onChange={e => updateRow(i, col, col === 'quantity' || col === 'rate' ? parseFloat(e.target.value) || 0 : e.target.value)}
                placeholder={col}
                aria-label={`${col} for line ${i + 1}`}
                className="h-8 text-sm"
              />
            ))}
            {rows.length > 1 && (
              <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" aria-label={`Remove line ${i + 1}`} title="Remove line" onClick={() => removeRow(i)}>
                <Trash2 className="h-3.5 w-3.5 text-destructive" />
              </Button>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

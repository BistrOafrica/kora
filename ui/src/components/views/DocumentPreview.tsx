import type { ViewComponentProps } from './registry'
import { Button } from '@/components/ui/button'
import { Printer, Share2 } from 'lucide-react'

/**
 * DocumentPreview renders a read-only formatted document summary
 * suitable for receipts, invoices, and purchase orders.
 */
export default function DocumentPreview(props: ViewComponentProps) {
  const { data, config, onAction } = props
  const doc = data?.data || data || {}
  const fields = Object.entries(doc).filter(([k]) => !k.startsWith('_') && k !== 'name')

  const formatValue = (value: unknown) => {
    if (value == null || value === '') return '—'
    if (Array.isArray(value)) return `${value.length} item${value.length === 1 ? '' : 's'}`
    if (typeof value === 'object') return Object.entries(value as Record<string, unknown>).slice(0, 3).map(([key, entry]) => `${key.replace(/_/g, ' ')}: ${entry ?? '—'}`).join(' · ')
    return String(value)
  }

  return (
    <div className="rounded-lg border bg-white dark:bg-card">
      <div className="flex items-center justify-between px-6 py-4 border-b bg-muted/10">
        <div>
          <h2 className="text-lg font-bold">{config.label || 'Document'}</h2>
          <p className="text-xs text-muted-foreground font-mono">{doc.name}</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" className="min-h-10" aria-label="Print document" onClick={() => onAction('print', doc)}><Printer className="h-4 w-4 mr-1" />Print</Button>
          <Button variant="outline" size="sm" className="min-h-10" aria-label="Share document" onClick={() => onAction('share', doc)}><Share2 className="h-4 w-4 mr-1" />Share</Button>
        </div>
      </div>
      <div className="p-6 space-y-3">
        {fields.map(([key, value]) => (
          <div key={key} className="flex justify-between text-sm border-b border-dashed pb-1">
            <span className="text-muted-foreground capitalize">{key.replace(/_/g, ' ')}</span>
            <span className="max-w-[70%] text-right font-medium">{formatValue(value)}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

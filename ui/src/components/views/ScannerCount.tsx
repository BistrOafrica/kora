import { useState, useRef, useEffect } from 'react'
import type { ViewComponentProps } from './registry'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Loader2, Check, X } from 'lucide-react'

interface CountEntry {
  barcode: string
  product_name: string
  expected: number
  counted: number
  variance: number
}

/**
 * ScannerCount is an inventory count screen. The user scans barcodes
 * repeatedly to increment counted quantities. Shows expected vs counted
 * and variance.
 */
export default function ScannerCount(props: ViewComponentProps) {
  const { data, isLoading, config, onAction } = props
  const [scanValue, setScanValue] = useState('')
  const [entries, setEntries] = useState<CountEntry[]>([])
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  // Initialize entries from data.
  useEffect(() => {
    if (data?.data && entries.length === 0) {
      const initial = (data.data as any[]).map((row: any) => ({
        barcode: row.barcode || row.sku || row.name,
        product_name: row.product_name || row.name,
        expected: parseInt(row.stock_qty || row.qty || '0'),
        counted: 0,
        variance: 0,
      }))
      setEntries(initial)
    }
  }, [data, entries.length])

  const handleScan = () => {
    if (!scanValue.trim()) return
    const barcode = scanValue.trim()
    setEntries((prev) =>
      prev.map((e) =>
        e.barcode === barcode
          ? { ...e, counted: e.counted + 1, variance: e.counted + 1 - e.expected }
          : e,
      ),
    )
    setScanValue('')
    inputRef.current?.focus()
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') handleScan()
  }

  const matched = entries.filter((e) => e.counted > 0).length
  const total = entries.length

  if (isLoading) {
    return <div className="flex items-center justify-center p-8"><Loader2 className="h-5 w-5 animate-spin" /></div>
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Input
          ref={inputRef}
          value={scanValue}
          onChange={(e) => setScanValue(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Scan barcode..."
          aria-label={config.label || 'Barcode count scanner input'}
          className="text-lg"
          autoFocus
        />
        <Button className="min-h-10" onClick={handleScan}>Scan</Button>
      </div>
      <p className="text-xs text-muted-foreground">
        {matched} of {total} items counted
      </p>
      <div className="space-y-1 max-h-[400px] overflow-y-auto">
        {entries.map((e) => (
          <div
            key={e.barcode}
            className={`flex items-center justify-between rounded-md px-3 py-2 text-sm ${
              e.counted > 0 ? 'bg-muted/20' : ''
            }`}
          >
            <span className="font-medium truncate">{e.product_name}</span>
            <div className="flex items-center gap-3 shrink-0">
              <span className="text-muted-foreground">Expected: {e.expected}</span>
              <span className={e.counted > 0 ? 'font-bold' : ''}>Counted: {e.counted}</span>
              {e.variance !== 0 && (
                <span className={`text-xs ${e.variance > 0 ? 'text-green-600' : 'text-red-600'}`}>
                  {e.variance > 0 ? '+' : ''}{e.variance}
                </span>
              )}
              {e.counted > 0 && e.variance === 0 ? (
                <Check className="h-4 w-4 text-green-500" />
              ) : e.counted > 0 ? (
                <X className="h-4 w-4 text-red-500" />
              ) : null}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

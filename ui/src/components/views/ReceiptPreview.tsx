import type { ViewComponentProps } from './registry'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Printer, Download } from 'lucide-react'

/**
 * ReceiptPreview shows a transaction summary with line items and totals.
 */
export default function ReceiptPreview(props: ViewComponentProps) {
  const { data, config, onAction } = props
  const sale = data?.data || data || {}
  const items = sale.items || sale.cart || []
  const total = sale.total || sale.grand_total || 0
  const payment = sale.payment_method || 'Cash'
  const date = sale.creation || sale.date || new Date().toISOString()

  return (
    <div className="max-w-sm mx-auto rounded-lg border bg-white dark:bg-card shadow-lg">
      <div className="px-6 py-4 text-center border-b">
        <h2 className="text-lg font-bold">{config.label || 'Receipt'}</h2>
        <p className="text-xs text-muted-foreground">{new Date(date).toLocaleString()}</p>
        <p className="text-xs font-mono text-muted-foreground mt-1">{sale.name}</p>
      </div>
      <div className="px-4 py-2 divide-y">
        {items.map((item: any, i: number) => (
          <div key={i} className="flex justify-between py-2 text-sm">
            <div className="flex-1 min-w-0">
              <p className="truncate">{item.product_name || item.name || 'Item'}</p>
              <p className="text-xs text-muted-foreground">{item.quantity} × {item.rate?.toLocaleString()}</p>
            </div>
            <span className="font-medium ml-4">{(item.amount || item.rate * item.quantity)?.toLocaleString()}</span>
          </div>
        ))}
      </div>
      <div className="px-4 py-3 border-t bg-muted/10 space-y-1">
        <div className="flex justify-between text-sm"><span>Subtotal</span><span>{total.toLocaleString()}</span></div>
        <div className="flex justify-between text-sm"><span>Payment</span><Badge variant="outline" className="text-xs">{payment}</Badge></div>
        <div className="flex justify-between text-base font-bold pt-1 border-t"><span>Total</span><span>{total.toLocaleString()}</span></div>
      </div>
      <div className="flex gap-2 px-4 py-3 border-t">
        <Button variant="outline" size="sm" className="flex-1" onClick={() => onAction('print', sale)}><Printer className="h-4 w-4 mr-1" />Print</Button>
        <Button variant="outline" size="sm" className="flex-1" onClick={() => onAction('download', sale)}><Download className="h-4 w-4 mr-1" />PDF</Button>
      </div>
    </div>
  )
}

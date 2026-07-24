import type { ViewComponentProps } from './registry'
import { useCartStore } from '@/lib/cart-store'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Minus, Plus, Trash2, ShoppingCart } from 'lucide-react'

/**
 * CartPanel shows line items, quantities, and totals from the Zustand cart store.
 * Uses local state — no server calls needed.
 */
export default function CartPanel(props: ViewComponentProps) {
  const { onAction } = props
  const { items, updateQuantity, removeItem, total, clearCart } = useCartStore()
  const cartTotal = total()

  if (items.length === 0) {
    return (
      <div className="rounded-lg border p-6 text-center">
        <ShoppingCart className="h-8 w-8 mx-auto text-muted-foreground mb-2" />
        <p className="text-sm text-muted-foreground">Cart is empty</p>
        <p className="text-xs text-muted-foreground/60 mt-1">Scan or tap products to add</p>
      </div>
    )
  }

  return (
    <div className="rounded-lg border">
      <div className="px-4 py-3 border-b bg-muted/20">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold">Cart ({items.length})</h3>
          <Button variant="ghost" size="sm" onClick={clearCart}>
            <Trash2 className="h-3.5 w-3.5 mr-1" />
            Clear
          </Button>
        </div>
      </div>
      <div className="divide-y max-h-[400px] overflow-y-auto">
        {items.map((item, i) => (
          <div key={item.product || item.name || i} className="px-4 py-3 flex items-center gap-3">
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium truncate">{item.product_name || item.product || item.name}</p>
              <p className="text-xs text-muted-foreground">@ {item.rate?.toLocaleString()}</p>
            </div>
            <div className="flex items-center gap-1">
              <Button
                variant="outline"
                size="icon"
                className="h-7 w-7"
                onClick={() => updateQuantity(item.product || item.name || '', Math.max(0, item.quantity - 1))}
              >
                <Minus className="h-3 w-3" />
              </Button>
              <span className="w-8 text-center text-sm font-medium">{item.quantity}</span>
              <Button
                variant="outline"
                size="icon"
                className="h-7 w-7"
                onClick={() => updateQuantity(item.product || item.name || '', item.quantity + 1)}
              >
                <Plus className="h-3 w-3" />
              </Button>
            </div>
            <Badge variant="secondary" className="text-xs">
              {(item.amount || item.rate * item.quantity).toLocaleString()}
            </Badge>
          </div>
        ))}
      </div>
      <div className="px-4 py-3 border-t bg-muted/10">
        <div className="flex items-center justify-between text-sm font-bold">
          <span>Total</span>
          <span>{cartTotal.toLocaleString()}</span>
        </div>
      </div>
    </div>
  )
}

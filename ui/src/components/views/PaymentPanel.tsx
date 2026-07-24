import { useState } from 'react'
import type { ViewComponentProps } from './registry'
import { Button } from '@/components/ui/button'
import { useCartStore } from '@/lib/cart-store'
import { Banknote, CreditCard, Smartphone, Loader2 } from 'lucide-react'

const PAYMENT_ICONS: Record<string, React.ReactNode> = {
  cash: <Banknote className="h-5 w-5" />,
  card: <CreditCard className="h-5 w-5" />,
  mobile_money: <Smartphone className="h-5 w-5" />,
  mobile: <Smartphone className="h-5 w-5" />,
}

/**
 * PaymentPanel renders payment method buttons and triggers the
 * complete_sale action when a method is selected.
 */
export default function PaymentPanel(props: ViewComponentProps) {
  const { config, onAction } = props
  const [processing, setProcessing] = useState(false)
  const { items, total, clearCart } = useCartStore()
  const cartTotal = total()

  const methods = (config.bindings?.methods || 'cash, card, mobile_money')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)

  const handlePayment = async (method: string) => {
    if (items.length === 0) return
    const actionId = config.actions?.find((action) => action.type === 'create_transaction')?.id
    if (!actionId) return

    setProcessing(true)
    try {
      await onAction(actionId, {
        cart: items,
        customer_name: 'Walk-in Customer',
        payment_status: 'Paid',
        payment_method: normalizePaymentMethod(method),
        total: cartTotal,
      })
      clearCart()
    } finally {
      setProcessing(false)
    }
  }

  return (
    <div className="rounded-lg border p-4 space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">Payment</h3>
        <span className="text-lg font-bold">{cartTotal.toLocaleString()}</span>
      </div>
      <div className="grid grid-cols-3 gap-2">
        {methods.map((method) => (
          <Button
            key={method}
            variant="outline"
            className="flex flex-col items-center gap-1 h-auto py-3"
            disabled={items.length === 0 || processing}
            onClick={() => handlePayment(method)}
          >
            {processing ? (
              <Loader2 className="h-5 w-5 animate-spin" />
            ) : (
              PAYMENT_ICONS[method] || <Banknote className="h-5 w-5" />
            )}
            <span className="text-xs capitalize">{method.replace('_', ' ')}</span>
          </Button>
        ))}
      </div>
    </div>
  )
}


function normalizePaymentMethod(method: string) {
  switch (method) {
    case 'cash':
      return 'Cash'
    case 'card':
      return 'Card'
    case 'mobile_money':
    case 'mobile':
      return 'Mobile Money'
    default:
      return method
        .split('_')
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(' ')
  }
}

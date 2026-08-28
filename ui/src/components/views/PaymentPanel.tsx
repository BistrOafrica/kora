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
  mpesa: <Smartphone className="h-5 w-5" />,
}

export default function PaymentPanel(props: ViewComponentProps) {
  const { config, onAction } = props
  const [processing, setProcessing] = useState(false)
  const [selectedMethod, setSelectedMethod] = useState<string | null>(null)
  const [phoneNumber, setPhoneNumber] = useState('')
  const [operationId, setOperationId] = useState('')
  const [paymentStatus, setPaymentStatus] = useState('')
  const [phoneError, setPhoneError] = useState('')
  const { items, total, clearCart } = useCartStore()
  const cartTotal = total()

  const methods = (config.bindings?.methods || 'cash, card, mobile_money')
    .split(',').map((s) => s.trim()).filter(Boolean)

  const transactionContext = (method: string) => ({
    cart: items,
    customer: config.bindings?.customer || 'CUST-0001',
    invoice_date: config.bindings?.invoice_date || new Date().toISOString().slice(0, 10),
    due_date: config.bindings?.due_date || new Date().toISOString().slice(0, 10),
    customer_name: 'Walk-in Customer',
    ...(isMobilePayment(method) ? { customer_phone: phoneNumber.trim() } : {}),
    payment_status: 'Paid',
    payment_method: normalizePaymentMethod(method),
    total: cartTotal,
  })

  const completeSale = async (method: string, externalOperation?: string) => {
    const action = config.actions?.find((item) => item.type === 'create_transaction' && (externalOperation ? item.config?.requires_operation_status : !item.config?.requires_operation_status))
    if (!action) return
    await onAction(action.id, { ...transactionContext(method), ...(externalOperation ? { external_operation: externalOperation } : {}) })
    clearCart()
    resetPayment()
  }

  const handlePayment = async (method: string) => {
    if (items.length === 0) return
    if (isMobilePayment(method)) {
      setSelectedMethod(method)
      setPhoneError('')
      return
    }
    setProcessing(true)
    try { await completeSale(method) } finally { setProcessing(false) }
  }

  const initiateMobilePayment = async () => {
    if (!selectedMethod || !isKenyanPhone(phoneNumber)) {
      setPhoneError('Enter a valid Kenyan phone number, for example 0712 345 678.')
      return
    }
    const action = config.actions?.find((item) => item.type === 'initiate_external_operation')
    if (!action) {
      setPhoneError('Mobile-money initiation is not configured for this POS.')
      return
    }
    setProcessing(true)
    try {
      const result = await onAction(action.id, {
        ...transactionContext(selectedMethod),
        client_reference: `pos-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      })
      const operation = result?.data || result
      setOperationId(operation?.name || '')
      setPaymentStatus(operation?.status || 'Pending')
    } finally { setProcessing(false) }
  }

  const validateMobilePayment = async () => {
    if (!operationId) return
    const action = config.actions?.find((item) => item.type === 'validate_external_operation')
    if (!action) return
    setProcessing(true)
    try {
      const result = await onAction(action.id, { operation_id: operationId })
      const operation = result?.data || result
      const status = operation?.status || 'Pending'
      setPaymentStatus(status)
      if (status === 'Succeeded') await completeSale(selectedMethod || 'mobile_money', operationId)
    } finally { setProcessing(false) }
  }

  const resetPayment = () => {
    setSelectedMethod(null)
    setPhoneNumber('')
    setOperationId('')
    setPaymentStatus('')
    setPhoneError('')
  }

  return (
    <div className="space-y-3 rounded-lg border p-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">Payment</h3>
        <span className="text-lg font-bold">{cartTotal.toLocaleString()}</span>
      </div>
      <div className="grid grid-cols-3 gap-2">
        {methods.map((method) => (
          <Button key={method} variant="outline" className="flex h-auto flex-col items-center gap-1 py-3" disabled={items.length === 0 || processing} onClick={() => handlePayment(method)}>
            {processing ? <Loader2 className="h-5 w-5 animate-spin" /> : PAYMENT_ICONS[method] || <Banknote className="h-5 w-5" />}
            <span className="text-xs capitalize">{method.replace('_', ' ')}</span>
          </Button>
        ))}
      </div>
      {selectedMethod && isMobilePayment(selectedMethod) && (
        <div className="space-y-2 rounded-md bg-muted/40 p-3">
          <label htmlFor="pos-payment-phone" className="text-sm font-medium">M-Pesa phone number</label>
          <input id="pos-payment-phone" type="tel" inputMode="tel" autoComplete="tel" placeholder="0712 345 678" value={phoneNumber} disabled={Boolean(operationId) || processing} onChange={(event) => { setPhoneNumber(event.target.value); setPhoneError('') }} className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring" />
          <p className="text-xs text-muted-foreground">Send the prompt, then validate the provider response.</p>
          {phoneError && <p className="text-xs text-destructive">{phoneError}</p>}
          {!operationId ? (
            <Button className="w-full" disabled={processing || !phoneNumber.trim()} onClick={initiateMobilePayment}>Send payment prompt</Button>
          ) : (
            <div className="space-y-2">
              <p className="text-sm">Payment status: <span className="font-medium">{paymentStatus || 'Pending'}</span></p>
              <div className="flex gap-2">
                <Button className="flex-1" disabled={processing} onClick={validateMobilePayment}>{processing ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}Validate payment</Button>
                <Button variant="outline" disabled={processing} onClick={resetPayment}>Cancel</Button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function isMobilePayment(method: string) {
  return ['mpesa', 'mobile_money', 'mobile'].includes(method.toLowerCase().replace(/\s+/g, '_'))
}

function isKenyanPhone(phone: string) {
  return /^(?:\+254|254|0)(?:1|7)\d{8}$/.test(phone.replace(/[\s-]/g, ''))
}

function normalizePaymentMethod(method: string) {
  switch (method) {
    case 'cash': return 'Cash'
    case 'card': return 'Card'
    case 'mobile_money':
    case 'mobile': return 'Mobile Money'
    case 'mpesa': return 'Mpesa'
    default: return method.split('_').map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ')
  }
}

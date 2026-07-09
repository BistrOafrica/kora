import { useEffect, useState, useCallback } from 'react'
import { CheckCircle, CircleAlert, Info, XCircle, X } from 'lucide-react'
import { cn } from '@/lib/utils'

type ToastType = 'success' | 'error' | 'info' | 'warning'

interface Toast {
  id: number
  type: ToastType
  message: string
  durationMs?: number
}

let toastId = 0
let addToastFn: ((type: ToastType, message: string, durationMs?: number) => void) | null = null

export function toast(type: ToastType, message: string, durationMs?: number) {
  addToastFn?.(type, message, durationMs)
}

export function ToastContainer() {
  const [toasts, setToasts] = useState<Toast[]>([])

  const addToast = useCallback((type: ToastType, message: string, durationMs = 4000) => {
    const id = ++toastId
    setToasts((prev) => [...prev.slice(-4), { id, type, message, durationMs }])
    window.setTimeout(() => {
      setToasts((prev) => prev.filter((toast) => toast.id !== id))
    }, durationMs)
  }, [])

  useEffect(() => {
    addToastFn = addToast
    return () => {
      addToastFn = null
    }
  }, [addToast])

  const dismiss = (id: number) => {
    setToasts((prev) => prev.filter((toast) => toast.id !== id))
  }

  return (
    <div className="fixed bottom-4 right-4 z-50 max-w-sm space-y-2">
      {toasts.map((toastItem) => (
        <div
          key={toastItem.id}
          className={cn(
            'flex items-center gap-2 rounded-lg px-4 py-3 text-sm shadow-lg animate-in slide-in-from-right',
            toastItem.type === 'success' && 'bg-green-700 text-white',
            toastItem.type === 'error' && 'bg-destructive text-destructive-foreground',
            toastItem.type === 'warning' && 'bg-amber-500 text-amber-950',
            toastItem.type === 'info' && 'border bg-card text-card-foreground',
          )}
        >
          {toastItem.type === 'success' && <CheckCircle className="h-4 w-4 shrink-0" />}
          {toastItem.type === 'error' && <XCircle className="h-4 w-4 shrink-0" />}
          {toastItem.type === 'warning' && <CircleAlert className="h-4 w-4 shrink-0" />}
          {toastItem.type === 'info' && <Info className="h-4 w-4 shrink-0" />}
          <span className="flex-1">{toastItem.message}</span>
          <button onClick={() => dismiss(toastItem.id)} className="shrink-0 opacity-70 hover:opacity-100">
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      ))}
    </div>
  )
}

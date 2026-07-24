import { useState } from 'react'
import type { ViewComponentProps } from './registry'
import { Button } from '@/components/ui/button'
import { ShieldAlert, Loader2 } from 'lucide-react'

/**
 * ConfirmationStep shows a role-protected "Are you sure?" modal
 * for manager-only actions like void, discount, or delete.
 */
export default function ConfirmationStep(props: ViewComponentProps) {
  const { config, onAction, disabled } = props
  const [open, setOpen] = useState(false)
  const [processing, setProcessing] = useState(false)

  const message = config.bindings?.message || 'Are you sure?'
  const confirmLabel = config.bindings?.confirm_label || 'Confirm'
  const roleRequired = config.bindings?.role_required

  const handleConfirm = async () => {
    setProcessing(true)
    try {
      await onAction('confirm', {})
      setOpen(false)
    } finally { setProcessing(false) }
  }

  return (
    <>
      <Button
        variant="destructive"
        size="sm"
        disabled={disabled}
        onClick={() => setOpen(true)}
      >
        <ShieldAlert className="h-4 w-4 mr-1" />
        {config.label || confirmLabel}
      </Button>

      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setOpen(false)}>
          <div className="bg-card rounded-lg border shadow-xl p-6 w-full max-w-sm mx-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-semibold">{config.label || 'Confirm Action'}</h3>
            <p className="mt-2 text-sm text-muted-foreground">{message}</p>
            {roleRequired && (
              <p className="mt-1 text-xs text-amber-600 dark:text-amber-400">Requires {roleRequired} role</p>
            )}
            <div className="mt-4 flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={() => setOpen(false)}>Cancel</Button>
              <Button variant="destructive" size="sm" disabled={processing} onClick={handleConfirm}>
                {processing ? <Loader2 className="h-4 w-4 mr-1 animate-spin" /> : null}
                {confirmLabel}
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}

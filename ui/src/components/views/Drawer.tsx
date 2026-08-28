import { useEffect, useState } from 'react'
import type { ViewComponentProps } from './registry'
import { X } from 'lucide-react'

/**
 * Drawer is a slide-out panel for mobile detail views.
 * Renders children inside a right-side slide-out overlay.
 */
export default function Drawer(props: ViewComponentProps) {
  const { config, children, onAction } = props
  const [open, setOpen] = useState(config.bindings?.open === 'true')

  useEffect(() => {
    if (open) document.body.style.overflow = 'hidden'
    else document.body.style.overflow = ''
    return () => { document.body.style.overflow = '' }
  }, [open])

  if (!open) {
    return (
      <button
        type="button"
        className="rounded-lg border border-dashed px-4 py-3 text-sm font-medium text-muted-foreground transition-colors hover:border-primary hover:text-foreground"
        aria-label={`Open ${config.label || 'detail'} drawer`}
        onClick={() => setOpen(true)}
      >
        Open {config.label || 'detail'}
      </button>
    )
  }

  return (
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-black/30" onClick={() => { setOpen(false); onAction('close', {}) }} />
      <div className="absolute right-0 top-0 bottom-0 w-full max-w-md bg-background shadow-xl overflow-y-auto" role="dialog" aria-modal="true" aria-label={config.label || 'Detail drawer'}>
        <div className="flex items-center justify-between px-4 py-3 border-b sticky top-0 bg-background z-10">
          <h3 className="text-sm font-semibold">{config.label || 'Detail'}</h3>
          <button aria-label="Close drawer" onClick={() => { setOpen(false); onAction('close', {}) }} className="min-h-11 min-w-11 rounded hover:bg-muted">
            <X className="h-5 w-5" />
          </button>
        </div>
        <div className="p-4">{children}</div>
      </div>
    </div>
  )
}

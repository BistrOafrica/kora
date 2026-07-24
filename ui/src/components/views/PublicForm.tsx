import { useState } from 'react'
import type { ViewComponentProps } from './registry'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Loader2, Send } from 'lucide-react'
import { api } from '@/lib/api/client'

/**
 * PublicForm renders an unauthenticated form for customer intake,
 * surveys, registrations, etc. Submits to POST /v?route=...
 * Only fields in the doctype's public_access.fields whitelist are accepted.
 */
export default function PublicForm(props: ViewComponentProps) {
  const { config } = props
  const [formData, setFormData] = useState<Record<string, any>>({})
  const [submitting, setSubmitting] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [error, setError] = useState('')

  const fields = config.bindings?.fields
    ? config.bindings.fields.split(',').map((s: string) => s.trim())
    : ['name', 'email']

  const labels: Record<string, string> = {}
  if (config.bindings?.labels) {
    try { Object.assign(labels, JSON.parse(config.bindings.labels)) } catch {}
  }

  const handleSubmit = async () => {
    setSubmitting(true)
    setError('')
    try {
      await api.post(`/api/v1/public/resource/${config.source_doctype}`, formData)
      setSubmitted(true)
    } catch (err: any) {
      setError(err.message || 'Submission failed')
    } finally {
      setSubmitting(false)
    }
  }

  if (submitted) {
    return (
      <div className="rounded-lg border p-8 text-center">
        <div className="text-4xl mb-3">✓</div>
        <h3 className="text-lg font-semibold">{config.bindings?.success_message || 'Thank you!'}</h3>
        <p className="text-sm text-muted-foreground mt-1">{config.bindings?.success_detail || 'Your submission has been received.'}</p>
      </div>
    )
  }

  return (
    <div className="max-w-md mx-auto rounded-lg border p-6 space-y-4">
      <h2 className="text-xl font-bold">{config.label || 'Form'}</h2>
      {config.bindings?.description && <p className="text-sm text-muted-foreground">{config.bindings.description}</p>}

      {fields.map((field: string) => (
        <div key={field} className="space-y-1.5">
          <Label>{labels[field] || field.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())}</Label>
          {field === 'message' || field === 'notes' || field === 'description' ? (
            <Textarea
              value={formData[field] || ''}
              onChange={e => setFormData({ ...formData, [field]: e.target.value })}
              rows={3}
            />
          ) : (
            <Input
              value={formData[field] || ''}
              onChange={e => setFormData({ ...formData, [field]: e.target.value })}
              type={field === 'email' ? 'email' : field === 'phone' ? 'tel' : 'text'}
            />
          )}
        </div>
      ))}

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Button className="w-full" disabled={submitting} onClick={handleSubmit}>
        {submitting ? <Loader2 className="h-4 w-4 mr-2 animate-spin" /> : <Send className="h-4 w-4 mr-2" />}
        {config.bindings?.submit_label || 'Submit'}
      </Button>
    </div>
  )
}

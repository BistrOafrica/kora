import { useEffect, useMemo, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useNavigate } from '@tanstack/react-router'
import type { Field, DocType } from '@/types/kora'
import { buildDefaultFormData, buildFormSections } from './form-runtime'

export interface DocumentFormProps {
  doctype: string
  label?: string
  onCreated?: (created: Record<string, any>) => void
  disabled?: boolean
  readonly?: boolean
}

export function DocumentForm({ doctype, label, onCreated, disabled = false, readonly = false }: DocumentFormProps) {
  const navigate = useNavigate()
  const [schema, setSchema] = useState<DocType | null>(null)
  const [formData, setFormData] = useState<Record<string, any>>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    void (async () => {
      setLoading(true)
      setError(null)
      try {
        const response = await fetch(`/api/v1/system/doctype/${encodeURIComponent(doctype)}`, { credentials: 'same-origin' })
        if (!response.ok) throw new Error(`Failed to load schema for ${doctype}`)
        const payload = await response.json()
        const next = payload.data?.doctype ?? payload.data ?? payload.doctype ?? null
        if (!cancelled) setSchema(next)
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load schema')
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [doctype])

  const fields = useMemo(() => schema?.fields?.filter((field) => !isLayoutField(field.fieldtype)) ?? [], [schema])
  const sections = useMemo(() => buildFormSections(schema?.fields ?? []), [schema])

  useEffect(() => {
    if (!schema) return
    setFormData(buildDefaultFormData(fields))
  }, [schema, fields])

  const handleFieldChange = (fieldname: string, value: any) => {
    setFormData((prev) => ({ ...prev, [fieldname]: value }))
  }

  const handleSubmit = async () => {
    if (!schema) return
    setSaving(true)
    setError(null)
    try {
      const response = await fetch(`/api/v1/resource/${encodeURIComponent(doctype)}`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData),
      })
      if (!response.ok) {
        const payload = await response.json().catch(() => ({}))
        throw new Error(payload?.error?.message || payload?.message || `Failed to create ${doctype}`)
      }
      const payload = await response.json()
      const created = payload.data ?? payload
      onCreated?.(created)
      if (created?.name) {
        navigate({ to: '/workspace/$doctype/$name', params: { doctype, name: created.name } })
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create record')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <div className="rounded-2xl border p-6 text-sm text-muted-foreground">Loading form…</div>
  }

  if (error) {
    return <div className="rounded-2xl border border-destructive/40 bg-destructive/5 p-6 text-sm text-destructive">{error}</div>
  }

  if (!schema) {
    return <div className="rounded-2xl border border-dashed p-6 text-sm text-muted-foreground">No schema found for {doctype}.</div>
  }

  const requiredFields = fields.filter((field) => field.reqd)
  const filledRequired = requiredFields.filter((field) => {
    const value = formData[field.fieldname]
    return value !== null && value !== undefined && value !== ''
  }).length

  return (
    <div className="rounded-2xl border bg-card shadow-sm">
      <div className="flex items-start justify-between gap-4 border-b px-4 py-4 md:px-6">
        <div>
          <div className="flex items-center gap-2">
            <h3 className="text-lg font-semibold">{label || `New ${schema.name}`}</h3>
            <span className="rounded-full border px-2 py-0.5 text-xs text-muted-foreground">
              {filledRequired}/{requiredFields.length || 0} required
            </span>
          </div>
          <p className="mt-1 text-sm text-muted-foreground">Create a new {schema.name.toLowerCase()} record.</p>
        </div>
        <button
          type="button"
          onClick={handleSubmit}
          disabled={saving || disabled || readonly}
          className="inline-flex items-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:cursor-not-allowed disabled:opacity-50"
        >
          {saving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
          {saving ? 'Saving...' : 'Create'}
        </button>
      </div>

      <div className="space-y-4 p-4 md:p-6">
        {sections.map((section, index) => (
          <section key={`${section.title}-${index}`} id={`section-${index}`} className="space-y-4 rounded-xl border p-4">
            <h4 className="text-sm font-semibold">{section.title}</h4>
            <div className="grid gap-4 md:grid-cols-2">
              {section.fields.map((field) => (
                <FormField
                  key={field.fieldname}
                  field={field}
                  value={formData[field.fieldname]}
                  onChange={handleFieldChange}
                  disabled={saving || disabled || readonly}
                />
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  )
}

function FormField({
  field,
  value,
  onChange,
  disabled,
}: {
  field: Field
  value: any
  onChange: (fieldname: string, value: any) => void
  disabled: boolean
}) {
  const label = `${field.label}${field.reqd ? ' *' : ''}`
  const common = {
    disabled: disabled || field.read_only,
    className: 'w-full rounded-md border bg-background px-3 py-2 text-sm',
  }

  if (field.fieldtype === 'Check') {
    return (
      <label className="flex items-center gap-3 rounded-md border px-3 py-2 text-sm">
        <input
          type="checkbox"
          checked={Boolean(value)}
          disabled={disabled || field.read_only}
          onChange={(event) => onChange(field.fieldname, event.target.checked)}
        />
        <span className="font-medium">{label}</span>
      </label>
    )
  }

  if (field.fieldtype === 'Select') {
    const options = String(field.options || '').split('\n').map((option) => option.trim()).filter(Boolean)
    return (
      <label className="space-y-1.5 text-sm">
        <span className="font-medium">{label}</span>
        <select
          {...common}
          value={value ?? ''}
          onChange={(event) => onChange(field.fieldname, event.target.value)}
        >
          <option value="">Select...</option>
          {options.map((option) => <option key={option} value={option}>{option}</option>)}
        </select>
      </label>
    )
  }

  if (field.fieldtype === 'Text' || field.fieldtype === 'Text Editor') {
    return (
      <label className="space-y-1.5 text-sm">
        <span className="font-medium">{label}</span>
        <textarea
          {...common}
          rows={field.fieldtype === 'Text Editor' ? 6 : 4}
          value={value ?? ''}
          onChange={(event) => onChange(field.fieldname, event.target.value)}
        />
      </label>
    )
  }

  const type = field.fieldtype === 'Int' || field.fieldtype === 'Float' || field.fieldtype === 'Currency' || field.fieldtype === 'Percent'
    ? 'number'
    : field.fieldtype === 'Date'
      ? 'date'
      : field.fieldtype === 'Datetime'
        ? 'datetime-local'
        : field.fieldtype === 'Time'
          ? 'time'
          : 'text'

  return (
    <label className="space-y-1.5 text-sm">
      <span className="font-medium">{label}</span>
      <input
        {...common}
        type={type}
        value={value ?? ''}
        onChange={(event) => onChange(field.fieldname, event.target.value)}
      />
      <span className="block text-xs text-muted-foreground">{field.description || field.fieldtype}</span>
    </label>
  )
}

function isLayoutField(fieldtype: string): boolean {
  return fieldtype === 'Section Break' || fieldtype === 'Column Break' || fieldtype === 'Heading'
}

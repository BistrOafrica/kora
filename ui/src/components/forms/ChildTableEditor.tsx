import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchDoctypeSchema } from '@/lib/api/system'
import { FieldRenderer } from './FieldRenderer'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Plus, Trash2 } from 'lucide-react'
import { applyComputedFields } from '@/lib/computed-fields'
import { cn } from '@/lib/utils'
import type { Field } from '@/types/kora'

interface ChildTableEditorProps {
  field: Field
  value: Record<string, any>[]
  onChange: (fieldname: string, value: Record<string, any>[]) => void
  onRowsChange?: (fieldname: string, rows: Record<string, any>[]) => void
  disabled: boolean
  errors?: Record<number, Record<string, string>>
}

export function ChildTableEditor({ field, value, onChange, onRowsChange, disabled, errors }: ChildTableEditorProps) {
  const childDoctype = field.options // "Order Item"
  const rows: Record<string, any>[] = value ?? []
  const [localRows, setLocalRows] = useState<Record<string, any>[]>(rows)

  // Fetch child doctype schema for its fields.
  const schemaQuery = useQuery({
    queryKey: ['doctype', childDoctype],
    queryFn: () => fetchDoctypeSchema(childDoctype),
    enabled: !!childDoctype,
    staleTime: 5 * 60_000,
  })

  const childFields: Field[] =
    schemaQuery.data?.doctype?.fields?.filter(
      (f: Field) =>
        !['Section Break', 'Column Break', 'Heading', 'Table'].includes(f.fieldtype),
    ) ?? []

  useEffect(() => {
    setLocalRows(Array.isArray(value) ? value : [])
  }, [value])

  const updateRows = (newRows: Record<string, any>[]) => {
    setLocalRows(newRows)
    onChange(field.fieldname, newRows)
    onRowsChange?.(field.fieldname, newRows)
  }

  const addRow = () => {
    const newRow: Record<string, any> = {}
    childFields.forEach((f) => {
      if (f.default) newRow[f.fieldname] = f.default
    })
    updateRows([...localRows, newRow])
  }

  const removeRow = (idx: number) => {
    updateRows(localRows.filter((_, i) => i !== idx))
  }

  const updateRow = (idx: number, fieldname: string, val: any) => {
    const updated = localRows.map((row, i) =>
      i === idx ? { ...row, [fieldname]: val } : row,
    )

    // Generic linked_field auto-population: when a Link field changes,
    // fetch the linked document and populate fields that reference it.
    const changedField = childFields.find((f) => f.fieldname === fieldname)
    if (changedField && (changedField.fieldtype === 'Link' || changedField.fieldtype === 'Dynamic Link') && val) {
      const targetDoctype = changedField.options
      const linkedFields = childFields.filter(
        (f) => f.linked_field?.startsWith(fieldname + '.'),
      )
      if (linkedFields.length > 0 && targetDoctype) {
        import('@/lib/api/resources').then(({ fetchDocument }) => {
          fetchDocument(targetDoctype, val).then((doc) => {
            setLocalRows((prev) => {
              const newRows = prev.map((row, i) => {
                if (i !== idx) return row
                const newRow = { ...row }
                for (const lf of linkedFields) {
                  const sourceField = lf.linked_field!.split('.')[1]
                  if (doc[sourceField] !== undefined) {
                    newRow[lf.fieldname] = doc[sourceField]
                  }
                }
                // Apply computed fields after linked fields are populated.
                return applyComputedFields(childFields as Field[], newRow)
              })
              // Notify parent of changes (triggers computed fields on parent).
              onChange(field.fieldname, newRows)
              onRowsChange?.(field.fieldname, newRows)
              return newRows
            })
          }).catch(() => {})
        })
      }
    }

    // Apply computed field expressions from config.
    const row = applyComputedFields(childFields as Field[], updated[idx])
    updated[idx] = row

    updateRows(updated)
  }

  if (schemaQuery.isLoading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-24 w-full" />
      </div>
    )
  }

  const rowErrors = (idx: number) => errors?.[idx] ?? {}

  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-0.5">
          <h4 className="text-sm font-semibold">{field.label}</h4>
          <p className="text-xs text-muted-foreground">
            {localRows.length === 0
              ? `No ${childDoctype || 'items'} yet`
              : `${localRows.length} ${localRows.length === 1 ? 'row' : 'rows'}`}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={addRow}
          disabled={disabled}
          type="button"
          className="w-full sm:w-auto"
        >
          <Plus className="mr-1 h-3.5 w-3.5" />
          Add Row
        </Button>
      </div>

      {localRows.length === 0 ? (
        <div className="rounded-xl border border-dashed bg-muted/20 p-5 text-center sm:p-8">
          <p className="text-sm font-medium">Start adding {childDoctype || 'items'}</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Each row opens as a mobile-friendly card with the row actions kept visible.
          </p>
          <Button
            variant="outline"
            size="sm"
            onClick={addRow}
            disabled={disabled}
            type="button"
            className="mt-4 w-full sm:w-auto"
          >
            <Plus className="mr-1 h-3.5 w-3.5" />
            Add first row
          </Button>
        </div>
      ) : (
        <div className="space-y-3 sm:space-y-2">
          {localRows.map((row, idx) => (
            <div
              key={idx}
              className={cn(
                'group relative rounded-xl border bg-background p-3 shadow-sm sm:rounded-lg sm:bg-muted/30 sm:px-3 sm:py-2 sm:shadow-none',
                Object.keys(rowErrors(idx)).length > 0 && 'border-destructive',
              )}
            >
              <div className="mb-3 flex items-center justify-between gap-3 sm:mb-0">
                <div className="min-w-0 sm:hidden">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    Row {idx + 1}
                  </p>
                  {Object.keys(rowErrors(idx)).length > 0 && (
                    <p className="text-xs text-destructive">Review highlighted fields</p>
                  )}
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-10 w-10 rounded-full text-muted-foreground hover:text-destructive sm:absolute sm:-right-1 sm:-top-1 sm:h-6 sm:w-6 sm:opacity-0 sm:transition-opacity sm:group-hover:opacity-100 sm:focus-visible:opacity-100"
                  onClick={() => removeRow(idx)}
                  disabled={disabled}
                  type="button"
                  aria-label={`Remove row ${idx + 1}`}
                >
                  <Trash2 className="h-3.5 w-3.5 sm:h-3 sm:w-3" />
                </Button>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-2 lg:grid-cols-4 items-start">
                {childFields.map((cf) => (
                  <FieldRenderer
                    key={cf.fieldname}
                    field={cf}
                    value={row[cf.fieldname] ?? null}
                    onChange={(fn, v) => updateRow(idx, fn, v)}
                    disabled={disabled}
                    error={rowErrors(idx)[cf.fieldname]}
                    compact
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

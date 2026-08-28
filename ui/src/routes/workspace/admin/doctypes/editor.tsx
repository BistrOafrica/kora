import { useNavigate, useRouterState } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useCallback, useRef, useEffect } from 'react'
import { fetchDoctypes, createDoctype, updateDoctype } from '@/lib/api/system'
import { createPageManifest } from '@/lib/api/page-manifests'
import { dryRunDoctype } from '@/lib/api/system'
import type { DocType, Field } from '@/types/kora'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { ArrowLeft, Plus, ChevronDown, ChevronRight, GripVertical, Edit, Trash2, Save } from 'lucide-react'
import { YamlPanel } from '@/components/forms/YamlPanel'
import { LispAutocomplete } from '@/components/forms/LispAutocomplete'
import { Badge } from '@/components/ui/badge'
import { FieldLabelWithHelp, HelpTooltip } from '@/components/ui/help-tooltip'
import { cn } from '@/lib/utils'
import { createStandardPageManifest, STANDARD_PAGE_KINDS, type StandardPageKind } from '@/manifest/runtime/standard-pages'
import { validateWizardDoctype, titleCase, slugField, type DraftIssue } from './editor-helpers'

// --- Field type groups ---
const FIELD_TYPE_GROUPS: { label: string; types: string[] }[] = [
  { label: 'Text', types: ['Data', 'Text', 'Text Editor', 'Password', 'JSON'] },
  { label: 'Numbers', types: ['Int', 'Float', 'Currency', 'Percent'] },
  { label: 'Date & Time', types: ['Date', 'Time', 'Datetime'] },
  { label: 'Selection', types: ['Select', 'Check'] },
  { label: 'Relations', types: ['Link', 'Dynamic Link', 'Table'] },
  { label: 'Files', types: ['Attach', 'Attach Image', 'Attach Audio'] },
  { label: 'Layout', types: ['Section Break', 'Column Break', 'Heading'] },
]

const EMPTY_FIELD: Field = {
  fieldname: '',
  fieldtype: 'Data' as any,
  label: '',
  options: '',
  reqd: false,
  unique: false,
  default: '',
  hidden: false,
  read_only: false,
  bold: false,
  in_list_view: false,
  in_standard_filter: false,
  search_index: false,
  description: '',
  depends_on: '',
  mandatory_depends_on: '',
  constraints: null,
  renamed_from: '',
  linked_field: '',
  computed: '',
}

const EMPTY_DOCTYPE: DocType = {
  name: '',
  module: '',
  is_submittable: false,
  is_child_table: false,
  is_single: false,
  track_changes: false,
  title_field: 'title',
  search_fields: 'title',
  sort_field: 'modified',
  sort_order: 'DESC',
  description: '',
  fields: [
    { ...EMPTY_FIELD, fieldname: 'title', fieldtype: 'Data', label: 'Title', reqd: true, in_list_view: true, search_index: true },
  ],
}

const WIZARD_STEPS = ['Basics', 'Fields', 'Screens', 'Review'] as const
type WizardStep = typeof WIZARD_STEPS[number]
type AssistantDraft = {
  reply: string
  fields: Array<[string, Field['fieldtype']]>
  screens: StandardPageKind[]
}

const STARTER_TEMPLATES: Array<{ label: string; description: string; fields: Array<[string, Field['fieldtype']]> }> = [
  {
    label: 'Customer record',
    description: 'Names, contacts, status, and notes.',
    fields: [['Customer name', 'Data'], ['Email', 'Data'], ['Phone', 'Data'], ['Status', 'Select'], ['Notes', 'Text']],
  },
  {
    label: 'Order workflow',
    description: 'Customer, due date, amount, and status.',
    fields: [['Customer', 'Link'], ['Order date', 'Date'], ['Due date', 'Date'], ['Status', 'Select'], ['Total', 'Currency']],
  },
  {
    label: 'Task tracker',
    description: 'Owner, priority, due date, and progress.',
    fields: [['Subject', 'Data'], ['Owner', 'Link'], ['Priority', 'Select'], ['Due date', 'Date'], ['Done', 'Check']],
  },
]

export default function AdminDoctypeEditorPage() {
  const navigate = useNavigate()
  const routerState = useRouterState()
  // Determine if editing from the URL path: .../doctypes/<name> vs .../doctypes/new
  const pathParts = routerState.location.pathname.replace(/\/$/, '').split('/')
  const lastSegment = pathParts[pathParts.length - 1]
  const isEdit = lastSegment !== 'new' && lastSegment !== 'doctypes'
  const doctypeName = isEdit ? decodeURIComponent(lastSegment) : undefined

  const [form, setForm] = useState<DocType>(EMPTY_DOCTYPE)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [expandedField, setExpandedField] = useState<number | null>(0)
  const [loadingEdit, setLoadingEdit] = useState(isEdit)
  const [currentStatus, setCurrentStatus] = useState<string | null>(null)
  const [wizardStep, setWizardStep] = useState<WizardStep>('Basics')
  const [selectedScreens, setSelectedScreens] = useState<StandardPageKind[]>(['table', 'form'])
  const [preflight, setPreflight] = useState<{
    status: 'idle' | 'loading' | 'ready' | 'error'
    result?: Awaited<ReturnType<typeof dryRunDoctype>>
    message?: string
  }>({ status: 'idle' })

  // For edit mode, load from the doctypes list.
  const { data: doctypes } = useQuery({
    queryKey: ['admin', 'doctypes'],
    queryFn: fetchDoctypes,
    enabled: true,
  })

  // Populate form when doctypes load in edit mode.
  useEffect(() => {
    if (isEdit && doctypes && loadingEdit) {
      const existing = doctypes.find((d: DocType) => d.name === doctypeName)
      if (existing) {
        setForm(JSON.parse(JSON.stringify(existing)))
        setCurrentStatus(existing.status || null)
        setLoadingEdit(false)
      }
    }
  }, [isEdit, doctypes, doctypeName, loadingEdit])

  const updateField = useCallback((index: number, updates: Partial<Field>) => {
    setForm((prev) => {
      const fields = [...prev.fields]
      fields[index] = { ...fields[index], ...updates }
      return { ...prev, fields }
    })
  }, [])

  const addField = useCallback(() => {
    setForm((prev) => ({
      ...prev,
      fields: [...prev.fields, { ...EMPTY_FIELD }],
    }))
    setExpandedField(form.fields.length)
  }, [form.fields.length])

  const removeField = useCallback((index: number) => {
    setForm((prev) => ({
      ...prev,
      fields: prev.fields.filter((_, i) => i !== index),
    }))
    setExpandedField(null)
  }, [])

  const moveField = useCallback((index: number, direction: -1 | 1) => {
    setForm((prev) => {
      const fields = [...prev.fields]
      const target = index + direction
      if (target < 0 || target >= fields.length) return prev
      ;[fields[index], fields[target]] = [fields[target], fields[index]]
      return { ...prev, fields }
    })
    setExpandedField(null)
  }, [])

  const queryClient = useQueryClient()

  const handleSave = async (activate: boolean) => {
    setSaving(true)
    setError(null)
    try {
      if (activate) {
        const preview = await dryRunDoctype(form)
        setPreflight({ status: 'ready', result: preview })
        if (preview.blocked.length > 0) {
          setError(formatPreviewBlockers(preview.blocked))
          return
        }
      }

      if (isEdit) {
        await updateDoctype(doctypeName!, form, activate)
      } else {
        await createDoctype(form, activate)
      }
      // Invalidate caches so the new/updated doctype appears immediately
      // in the admin list, sidebar navigation, and dashboard.
      queryClient.invalidateQueries({ queryKey: ['admin', 'doctypes'] })
      queryClient.invalidateQueries({ queryKey: ['navigation'] })
      navigate({ to: '/workspace/admin/doctypes' })
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  const handleWizardCreate = async () => {
    const validation = validateWizardDoctype(form, doctypes ?? [])
    if (validation.length > 0) {
      setError(validation.map((issue) => issue.message).join(' '))
      return
    }

    setSaving(true)
    setError(null)
    try {
      const normalized = normalizeWizardDoctype(form)
      await createDoctype(normalized, true)

      for (const screen of selectedScreens) {
        const manifest = createStandardPageManifest(normalized, screen)
        await createPageManifest(manifest)
      }

      queryClient.invalidateQueries({ queryKey: ['admin', 'doctypes'] })
      queryClient.invalidateQueries({ queryKey: ['page-manifests'] })
      queryClient.invalidateQueries({ queryKey: ['navigation'] })

      if (selectedScreens.length > 0) {
        const first = createStandardPageManifest(normalized, selectedScreens[0])
        navigate({ to: '/workspace/admin/page-manifests/$name', params: { name: first.metadata.name } })
      } else {
        navigate({ to: '/workspace/admin/doctypes' })
      }
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  if (isEdit && loadingEdit) {
    return (
      <div className="p-8 space-y-4">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-96 w-full" />
      </div>
    )
  }

  if (!isEdit) {
    return (
      <DoctypeCreationWizard
        form={form}
        setForm={setForm}
        step={wizardStep}
        setStep={setWizardStep}
        selectedScreens={selectedScreens}
        setSelectedScreens={setSelectedScreens}
        doctypes={doctypes ?? []}
        error={error}
        saving={saving}
        issues={validateWizardDoctype(form, doctypes ?? [])}
        onCancel={() => navigate({ to: '/workspace/admin/doctypes' })}
        onCreate={handleWizardCreate}
      />
    )
  }

  return (
    <div className="h-[calc(100vh-4rem)] flex flex-col overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-3 sm:px-6 py-3 border-b shrink-0 gap-2">
        <div className="flex items-center gap-2 sm:gap-4 min-w-0">
            <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" aria-label="Back to DocTypes" title="Back to DocTypes" onClick={() => navigate({ to: '/workspace/admin/doctypes' })}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-base sm:text-2xl font-bold tracking-tight truncate">
            {isEdit ? doctypeName : 'New'}
          </h1>
          {currentStatus && (
            currentStatus === 'Active' ? (
              <Badge variant="default" className="bg-green-600 hover:bg-green-600 shrink-0">Active</Badge>
            ) : (
              <Badge variant="secondary" className="bg-amber-100 text-amber-800 hover:bg-amber-100 shrink-0">Draft</Badge>
            )
          )}
        </div>
        <div className="flex gap-1 sm:gap-2 shrink-0">
          <Button variant="outline" size="sm" className="text-xs sm:text-sm h-8" onClick={() => handleSave(false)} disabled={saving}>
            Save draft
          </Button>
          <Button size="sm" className="text-xs sm:text-sm h-8" onClick={() => handleSave(true)} disabled={saving}>
            <Save className="h-3.5 w-3.5 sm:mr-1" />
            <span className="hidden sm:inline">{currentStatus === 'Active' ? 'Save & Migrate' : 'Save & Activate'}</span>
          </Button>
        </div>
      </div>

      <div className="mx-4 mt-2 space-y-2">
        {error && (
          <div className="p-3 border border-destructive/50 bg-destructive/10 rounded-lg text-sm text-destructive">
            {error}
          </div>
        )}
        {preflight.status === 'ready' && preflight.result && (
          <PreflightSummary preview={preflight.result} />
        )}
      </div>

      {/* Split pane: Form (left) + YAML (right) */}
      <div className="flex-1 flex overflow-hidden">
        {/* Form panel */}
        <div className="flex-1 overflow-y-auto p-4 sm:p-6 lg:p-8 space-y-6 sm:space-y-8">
        {/* Doctype Properties */}
        <section>
          <h2 className="text-lg font-semibold mb-4">Basic details</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <Label htmlFor="name">Data object name *</Label>
              <Input
                id="name"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="Invoice"
                disabled={isEdit}
              />
            </div>
            <div>
              <Label htmlFor="module">Area *</Label>
              <Input
                id="module"
                value={form.module}
                onChange={(e) => setForm({ ...form, module: e.target.value })}
                placeholder="Billing"
              />
            </div>
            <div>
              <Label htmlFor="title_field">Main label field</Label>
              <Input
                id="title_field"
                value={form.title_field}
                onChange={(e) => setForm({ ...form, title_field: e.target.value })}
              />
            </div>
            <div>
              <Label htmlFor="search_fields">Searchable fields</Label>
              <Input
                id="search_fields"
                value={form.search_fields}
                onChange={(e) => setForm({ ...form, search_fields: e.target.value })}
                placeholder="name, email"
              />
            </div>
            <div>
              <Label htmlFor="sort_field">Sort by</Label>
              <Input
                id="sort_field"
                value={form.sort_field}
                onChange={(e) => setForm({ ...form, sort_field: e.target.value })}
              />
            </div>
            <div>
              <Label htmlFor="sort_order">Sort direction</Label>
              <Select value={form.sort_order} onValueChange={(v) => setForm({ ...form, sort_order: v || 'DESC' })}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ASC">ASC</SelectItem>
                  <SelectItem value="DESC">DESC</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="grid grid-cols-2 sm:flex sm:flex-wrap gap-2 sm:gap-5 mt-4">
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={form.is_submittable} onCheckedChange={(v) => setForm({ ...form, is_submittable: v })} />
              <span className="truncate">Needs approval</span>
            </label>
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={form.is_child_table} onCheckedChange={(v) => setForm({ ...form, is_child_table: v })} />
              <span className="truncate">Belongs inside another record</span>
            </label>
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={form.is_single} onCheckedChange={(v) => setForm({ ...form, is_single: v })} />
              <span className="truncate">Only one record</span>
            </label>
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={form.track_changes} onCheckedChange={(v) => setForm({ ...form, track_changes: v })} />
              <span className="truncate">Keep change history</span>
            </label>
          </div>
        </section>

        <Separator />

        {/* Fields */}
        <section>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold">Fields</h2>
            <Button variant="outline" size="sm" onClick={addField}>
              <Plus className="h-4 w-4 mr-1" /> Add Field
            </Button>
          </div>
          <p className="text-sm text-muted-foreground mb-3">Give each field a clear label. Kora will keep the internal field key in sync for you.</p>

          <div className="space-y-1">
            {form.fields.map((field, index) => (
              <FieldRow
                key={index}
                field={field}
                index={index}
                expanded={expandedField === index}
                onToggle={() => setExpandedField(expandedField === index ? null : index)}
                onChange={(updates) => updateField(index, updates)}
                onRemove={() => removeField(index)}
                onMoveUp={() => moveField(index, -1)}
                onMoveDown={() => moveField(index, 1)}
                canMoveUp={index > 0}
                canMoveDown={index < form.fields.length - 1}
                allDoctypes={doctypes?.map((d: DocType) => d.name) || []}
              />
            ))}
          </div>
        </section>

        <Separator />

        {/* Document Constraints */}
        <section>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold">Checks and rules</h2>
            <Button variant="outline" size="sm" onClick={() => {
              setForm({
                ...form,
                doc_constraints: [...(form.doc_constraints || []), { type: 'Predicate', predicate: '', condition: '', message: '' }]
              })
            }}>
              <Plus className="h-4 w-4 mr-1" /> Add Constraint
            </Button>
          </div>
          {(form.doc_constraints || []).length === 0 && (
            <p className="text-sm text-muted-foreground italic">No rules added yet.</p>
          )}
          {(form.doc_constraints || []).map((c, ci) => (
            <div key={ci} className="border rounded-lg p-3 mb-2">
              <div className="grid grid-cols-12 gap-2 items-start">
                <div className="col-span-3">
                      <Label className="text-xs">Rule type</Label>
                  <select
                    className="w-full h-9 rounded-md border bg-background px-2 text-sm"
                    value={c.type}
                    onChange={(e) => {
                      const updated = [...(form.doc_constraints || [])]
                      updated[ci] = { ...updated[ci], type: e.target.value }
                      setForm({ ...form, doc_constraints: updated })
                    }}
                  >
                    <option value="Predicate">Predicate</option>
                    <option value="max">max</option>
                    <option value="min">min</option>
                    <option value="max_length">max_length</option>
                    <option value="min_length">min_length</option>
                    <option value="regex">regex</option>
                    <option value="one_of">one_of</option>
                    <option value="not_one_of">not_one_of</option>
                    <option value="min_date">min_date</option>
                    <option value="max_date">max_date</option>
                  </select>
                </div>

                {c.type === 'Predicate' ? (
                  <>
                    <div className="col-span-4">
                      <Label className="text-xs">Rule expression</Label>
                      <LispAutocomplete
                        className="h-9 text-sm font-mono"
                        value={c.predicate || ''}
                        onChange={(val) => {
                          const updated = [...(form.doc_constraints || [])]
                          updated[ci] = { ...updated[ci], predicate: val }
                          setForm({ ...form, doc_constraints: updated })
                        }}
                        placeholder="(> end_date start_date)"
                        fieldNames={form.fields?.map((f: any) => f.fieldname) || []}
                      />
                    </div>
                    <div className="col-span-4">
                      <Label className="text-xs">Apply only when</Label>
                      <LispAutocomplete
                        className="h-9 text-sm font-mono"
                        value={c.condition || ''}
                        onChange={(val) => {
                          const updated = [...(form.doc_constraints || [])]
                          updated[ci] = { ...updated[ci], condition: val }
                          setForm({ ...form, doc_constraints: updated })
                        }}
                        placeholder='doc.type == "wholesale"'
                        fieldNames={form.fields?.map((f: any) => f.fieldname) || []}
                      />
                    </div>
                    <div className="col-span-1 flex items-end pb-1">
                      <Button
                        variant="ghost" size="sm"
                        className="h-9 text-destructive w-full"
                        onClick={() => {
                          const updated = (form.doc_constraints || []).filter((_, i) => i !== ci)
                          setForm({ ...form, doc_constraints: updated.length > 0 ? updated : undefined })
                        }}
                      >✕</Button>
                    </div>
                  </>
                ) : (
                  <>
                    <div className="col-span-3">
                      <Label className="text-xs">Rule value</Label>
                      <Input
                        className="h-9 text-sm"
                        value={c.value != null ? String(c.value) : ''}
                        onChange={(e) => {
                          const v = e.target.value
                          const num = Number(v)
                          const updated = [...(form.doc_constraints || [])]
                          updated[ci] = { ...updated[ci], value: isNaN(num) ? v : num }
                          setForm({ ...form, doc_constraints: updated })
                        }}
                        placeholder="value"
                      />
                    </div>
                    <div className="col-span-5">
                      <Label className="text-xs">User-facing message</Label>
                      <Input
                        className="h-9 text-sm"
                        value={c.message || ''}
                        onChange={(e) => {
                          const updated = [...(form.doc_constraints || [])]
                          updated[ci] = { ...updated[ci], message: e.target.value }
                          setForm({ ...form, doc_constraints: updated })
                        }}
                        placeholder="Error message"
                      />
                    </div>
                    <div className="col-span-1 flex items-end pb-1">
                      <Button
                        variant="ghost" size="sm"
                        className="h-9 text-destructive w-full"
                        onClick={() => {
                          const updated = (form.doc_constraints || []).filter((_, i) => i !== ci)
                          setForm({ ...form, doc_constraints: updated.length > 0 ? updated : undefined })
                        }}
                      >✕</Button>
                    </div>
                  </>
                )}
              </div>

              {c.type === 'Predicate' && (
                <div className="mt-2">
                  <Label className="text-xs">User-facing message</Label>
                  <Input
                    className="h-9 text-sm"
                    value={c.message || ''}
                    onChange={(e) => {
                      const updated = [...(form.doc_constraints || [])]
                      updated[ci] = { ...updated[ci], message: e.target.value }
                      setForm({ ...form, doc_constraints: updated })
                    }}
                    placeholder="End date must be after start date"
                  />
                </div>
              )}
            </div>
          ))}
        </section>
      </div>

      {/* YAML panel (desktop only) */}
      <div className="hidden md:block w-[42%] border-l overflow-hidden shrink-0">
        <YamlPanel form={form} onApply={(parsed) => setForm({ ...form, ...parsed })} />
      </div>
    </div>
    </div>
  )
}

function DoctypeCreationWizard({
  form,
  setForm,
  step,
  setStep,
  selectedScreens,
  setSelectedScreens,
  doctypes,
  error,
  issues,
  saving,
  onCancel,
  onCreate,
}: {
  form: DocType
  setForm: (value: DocType | ((previous: DocType) => DocType)) => void
  step: WizardStep
  setStep: (step: WizardStep) => void
  selectedScreens: StandardPageKind[]
  setSelectedScreens: (screens: StandardPageKind[]) => void
  doctypes: DocType[]
  error: string | null
  issues: DraftIssue[]
  saving: boolean
  onCancel: () => void
  onCreate: () => void
}) {
  const stepIndex = WIZARD_STEPS.indexOf(step)
  const progress = Math.round(((stepIndex + 1) / WIZARD_STEPS.length) * 100)
  const canContinue = !validateWizardStep(form, step)
  const stepTitle: Record<WizardStep, string> = {
    Basics: 'Name the data object',
    Fields: 'Add the fields',
    Screens: 'Pick the screens',
    Review: 'Review',
  }

  const goNext = () => {
    if (!canContinue) return
    setStep(WIZARD_STEPS[Math.min(stepIndex + 1, WIZARD_STEPS.length - 1)])
  }

  return (
    <div className="min-h-[calc(100vh-4rem)] bg-muted/20">
      <header className="border-b bg-background px-4 py-3 md:px-6">
        <div className="mx-auto flex max-w-5xl items-center justify-between gap-4">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">Data objects</p>
            <h1 className="text-2xl font-semibold tracking-tight">Create data object</h1>
          </div>
          <Button variant="ghost" onClick={onCancel}>Cancel</Button>
        </div>
      </header>

      <main className="mx-auto grid max-w-5xl gap-4 p-4 lg:grid-cols-[190px_minmax(0,1fr)] md:p-6">
        <aside className="space-y-3">
          <Card className="shadow-none">
            <CardHeader className="space-y-3 pb-2">
              <CardTitle className="text-sm">Step {stepIndex + 1} of {WIZARD_STEPS.length}</CardTitle>
              <div className="h-1.5 rounded-full bg-muted">
                <div className="h-full rounded-full bg-foreground transition-all" style={{ width: `${progress}%` }} />
              </div>
            </CardHeader>
            <CardContent className="space-y-1.5">
              {WIZARD_STEPS.map((entry, index) => (
                <button
                  key={entry}
                  type="button"
                  className={cn(
                    'flex w-full items-center gap-3 rounded-md px-2.5 py-2 text-left text-sm transition-colors',
                    entry === step ? 'bg-foreground text-background' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                    index > stepIndex + 1 && 'opacity-50',
                  )}
                  disabled={index > stepIndex + 1}
                  onClick={() => setStep(entry)}
                >
                  <span className={cn(
                    'flex h-5 w-5 items-center justify-center rounded-full border text-[11px] font-semibold',
                    entry === step ? 'border-background/40 bg-background text-foreground' : index < stepIndex ? 'border-foreground bg-foreground text-background' : 'bg-background',
                  )}>
                    {index + 1}
                  </span>
                  <span>{entry}</span>
                </button>
              ))}
            </CardContent>
          </Card>
        </aside>

        <section className="space-y-4">
          {(error || issues.length > 0) && (
            <div className="space-y-2 rounded-lg border border-amber-500/30 bg-amber-50 p-3 text-sm text-amber-950">
              {error && <p className="text-destructive">{error}</p>}
              {issues.length > 0 && (
                <ul className="space-y-1">
                  {issues.map((issue, index) => (
                    <li key={`${issue.field ?? 'general'}-${index}`}>- {issue.message}</li>
                  ))}
                </ul>
              )}
            </div>
          )}

          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="text-lg font-semibold tracking-tight">{stepTitle[step]}</h2>
              <p className="text-sm text-muted-foreground">{form.name || 'Untitled object'}</p>
            </div>
            <div className="flex flex-wrap gap-2">
              <Badge variant="outline">{form.fields.filter((field) => field.fieldname).length} fields</Badge>
              <Badge variant="outline">{selectedScreens.length} screens</Badge>
            </div>
          </div>

          <WizardAssistantPanel
            form={form}
            setForm={setForm}
            setSelectedScreens={setSelectedScreens}
            setStep={setStep}
          />

          {step === 'Basics' && (
            <Card className="shadow-none">
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2">
                  What are you tracking?
                  <HelpTooltip label="Basics help">Use business words. Kora handles the safe internal doctype name.</HelpTooltip>
                </CardTitle>
              </CardHeader>
              <CardContent className="grid gap-3 md:grid-cols-2">
                <div className="space-y-1.5">
                  <FieldLabelWithHelp label="Name" help="The thing users will recognize in navigation, like Customer Visit or Invoice." />
                  <Input
                    value={form.name}
                    onChange={(event) => setForm((previous) => ({
                      ...previous,
                      name: titleCase(event.target.value),
                    }))}
                    placeholder="Customer Visit"
                    autoFocus
                  />
                </div>
                <div className="space-y-1.5">
                  <FieldLabelWithHelp label="Area" help="Groups this object with related screens in the workspace." />
                  <Input
                    value={form.module}
                    onChange={(event) => setForm((previous) => ({ ...previous, module: titleCase(event.target.value) }))}
                    placeholder="Field Service"
                  />
                </div>
                <div className="space-y-1.5 md:col-span-2">
                  <FieldLabelWithHelp label="Description" help="Optional. Keep it short; it is for admin context, not customer-facing copy." />
                  <Input
                    value={form.description}
                    onChange={(event) => setForm((previous) => ({ ...previous, description: event.target.value }))}
                    placeholder="Tracks visits, outcomes, and next actions."
                  />
                </div>
                <label className="flex items-center gap-2 rounded-lg border bg-muted/20 p-3 text-sm">
                  <Switch checked={form.track_changes} onCheckedChange={(value) => setForm((previous) => ({ ...previous, track_changes: value }))} />
                  <span className="flex items-center gap-1.5">
                    Audit trail
                    <HelpTooltip label="Audit trail help">Records who changed this data and when.</HelpTooltip>
                  </span>
                </label>
                <label className="flex items-center gap-2 rounded-lg border bg-muted/20 p-3 text-sm">
                  <Switch checked={form.is_submittable} onCheckedChange={(value) => setForm((previous) => ({ ...previous, is_submittable: value }))} />
                  <span className="flex items-center gap-1.5">
                    Approval step
                    <HelpTooltip label="Approval step help">Use when records must be submitted before they are final.</HelpTooltip>
                  </span>
                </label>
              </CardContent>
            </Card>
          )}

          {step === 'Fields' && (
            <Card className="shadow-none">
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2">
                  What information should it collect?
                  <HelpTooltip label="Fields help">Start with what users fill in. Advanced rules can be added later.</HelpTooltip>
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="grid gap-2 md:grid-cols-3">
                  {STARTER_TEMPLATES.map((template) => (
                    <button
                      key={template.label}
                      type="button"
                      className="rounded-lg border bg-muted/20 p-3 text-left transition-colors hover:border-foreground hover:bg-muted/40"
                      onClick={() => setForm((previous) => applyStarterTemplate(previous, template.fields))}
                    >
                      <p className="flex items-center gap-1.5 text-sm font-medium">
                        {template.label}
                        <HelpTooltip label={`${template.label} template help`}>{template.description}</HelpTooltip>
                      </p>
                    </button>
                  ))}
                </div>
                <div className="space-y-2">
                  {form.fields.map((field, index) => (
                    <SimpleFieldRow
                      key={index}
                      field={field}
                      index={index}
                      canRemove={form.fields.length > 1}
                      allDoctypes={doctypes.map((doctype) => doctype.name)}
                      onChange={(patch) => setForm((previous) => updateWizardField(previous, index, patch))}
                      onRemove={() => setForm((previous) => ({ ...previous, fields: previous.fields.filter((_, i) => i !== index) }))}
                    />
                  ))}
                </div>
                <Button variant="outline" onClick={() => setForm((previous) => ({
                  ...previous,
                  fields: [...previous.fields, createWizardField('New field')],
                }))}>
                  <Plus className="mr-2 h-4 w-4" />
                  Add field
                </Button>
              </CardContent>
            </Card>
          )}

          {step === 'Screens' && (
            <Card className="shadow-none">
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2">
                  Which screens should Kora create?
                  <HelpTooltip label="Screens help">These are generated page manifests connected to this doctype.</HelpTooltip>
                </CardTitle>
              </CardHeader>
              <CardContent className="grid gap-2 md:grid-cols-2">
                {STANDARD_PAGE_KINDS.map((screen) => {
                  const selected = selectedScreens.includes(screen.kind)
                  return (
                    <button
                      key={screen.kind}
                      type="button"
                      className={cn(
                        'rounded-lg border p-4 text-left transition-colors',
                        selected ? 'border-foreground bg-foreground text-background' : 'bg-card hover:bg-muted/40',
                      )}
                      onClick={() => setSelectedScreens(toggleScreen(selectedScreens, screen.kind))}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <p className="flex items-center gap-1.5 font-medium">
                            {screen.label}
                            <HelpTooltip label={`${screen.label} help`}>{screen.description}</HelpTooltip>
                          </p>
                        </div>
                        <Badge variant={selected ? 'secondary' : 'outline'}>{selected ? 'Included' : 'Optional'}</Badge>
                      </div>
                    </button>
                  )
                })}
              </CardContent>
            </Card>
          )}

          {step === 'Review' && (
            <Card className="shadow-none">
              <CardHeader className="pb-2">
                <CardTitle>Review and create</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-3 md:grid-cols-3">
                  <SummaryCard label="Data object" value={form.name || 'Missing'} />
                  <SummaryCard label="Fields" value={`${form.fields.filter((field) => field.fieldname).length}`} />
                  <SummaryCard label="Screens" value={`${selectedScreens.length}`} />
                </div>
                <div className="rounded-xl border bg-muted/20 p-4">
                  <p className="text-sm font-medium">Created screens</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {selectedScreens.length ? selectedScreens.map((screen) => (
                      <Badge key={screen} variant="secondary">{screen}</Badge>
                    )) : <span className="text-sm text-muted-foreground">No screens selected.</span>}
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          <div className="flex items-center justify-between rounded-lg border bg-background p-3">
            <Button variant="ghost" disabled={stepIndex === 0 || saving} onClick={() => setStep(WIZARD_STEPS[Math.max(stepIndex - 1, 0)])}>
              Back
            </Button>
            <div className="flex gap-2">
              {step !== 'Review' ? (
                <Button disabled={!canContinue || saving} onClick={goNext}>Continue</Button>
              ) : (
                <Button disabled={issues.length > 0 || saving} onClick={onCreate}>
                  {saving ? 'Creating...' : 'Create data object and screens'}
                </Button>
              )}
            </div>
          </div>
        </section>
      </main>
    </div>
  )
}

function SimpleFieldRow({
  field,
  index,
  canRemove,
  allDoctypes,
  onChange,
  onRemove,
}: {
  field: Field
  index: number
  canRemove: boolean
  allDoctypes: string[]
  onChange: (patch: Partial<Field>) => void
  onRemove: () => void
}) {
  return (
    <div className="grid gap-2 rounded-lg border bg-card p-3 md:grid-cols-[1.1fr_160px_1fr_auto]">
      <div className="space-y-1.5">
        <Label className="text-xs">Field label</Label>
        <Input
          value={field.label}
          onChange={(event) => {
            const label = event.target.value
            onChange({ label, fieldname: slugField(label) })
          }}
          placeholder={index === 0 ? 'Title' : 'Amount'}
        />
      </div>
      <div className="space-y-1.5">
        <Label className="text-xs">Type</Label>
        <Select value={field.fieldtype} onValueChange={(value) => onChange({ fieldtype: value as Field['fieldtype'], options: value === 'Select' ? field.options || 'Open\nClosed' : field.options })}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            {['Data', 'Text', 'Int', 'Currency', 'Check', 'Date', 'Datetime', 'Select', 'Link', 'Attach', 'Attach Image', 'Attach Audio'].map((type) => (
              <SelectItem key={type} value={type}>{friendlyFieldType(type)}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1.5">
        <Label className="text-xs">{field.fieldtype === 'Link' ? 'Links to' : field.fieldtype === 'Select' ? 'Choices' : 'Field key'}</Label>
        {field.fieldtype === 'Link' ? (
          <Select value={field.options} onValueChange={(value) => onChange({ options: value || '' })}>
            <SelectTrigger><SelectValue placeholder="Choose data object" /></SelectTrigger>
            <SelectContent>{allDoctypes.map((name) => <SelectItem key={name} value={name}>{name}</SelectItem>)}</SelectContent>
          </Select>
        ) : (
          <Input
            value={field.fieldtype === 'Select' ? field.options : field.fieldname}
            onChange={(event) => onChange(field.fieldtype === 'Select' ? { options: event.target.value } : { fieldname: slugField(event.target.value) })}
            placeholder={field.fieldtype === 'Select' ? 'Open, Closed' : 'field_key'}
          />
        )}
      </div>
      <div className="flex items-end gap-2">
        <label className="mb-2 flex items-center gap-1.5 text-xs">
          <Switch checked={field.reqd} onCheckedChange={(value) => onChange({ reqd: value })} />
          Required
        </label>
        {canRemove && (
          <Button variant="ghost" size="icon" className="mb-1 text-destructive" onClick={onRemove}>
            <Trash2 className="h-4 w-4" />
          </Button>
        )}
      </div>
    </div>
  )
}

function SummaryCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border bg-muted/20 p-4">
      <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</p>
      <p className="mt-2 text-lg font-semibold">{value}</p>
    </div>
  )
}

function WizardAssistantPanel({
  form,
  setForm,
  setSelectedScreens,
  setStep,
}: {
  form: DocType
  setForm: (value: DocType | ((previous: DocType) => DocType)) => void
  setSelectedScreens: (screens: StandardPageKind[]) => void
  setStep: (step: WizardStep) => void
}) {
  const [prompt, setPrompt] = useState('')
  const [material, setMaterial] = useState('')
  const [materialNote, setMaterialNote] = useState<string | null>(null)
  const [draft, setDraft] = useState<AssistantDraft | null>(null)
  const [expanded, setExpanded] = useState(false)
  const [showImport, setShowImport] = useState(false)

  const generateDraft = () => {
    const next = material.trim() ? inferMaterialDraft(material, form) : inferAssistantDraft(prompt, form)
    setDraft(next)
  }

  const handleMaterialFile = async (file: File | undefined) => {
    if (!file) return
    if (file.type.startsWith('image/')) {
      setMaterialNote('Image received. OCR extraction needs the typed backend AI endpoint; paste visible labels for now.')
      return
    }
    const text = await file.text()
    setMaterial(text.slice(0, 12_000))
    setMaterialNote(`Loaded ${file.name}. Review the extracted draft before applying.`)
  }

  const applyDraft = () => {
    if (!draft) return
    setForm((previous) => {
      const base = previous.name.trim() ? previous : {
        ...previous,
        name: inferObjectName(prompt),
        module: previous.module || inferModule(prompt),
      }
      return applyStarterTemplate(base, draft.fields)
    })
    setSelectedScreens(draft.screens)
    setStep('Fields')
  }

  return (
    <Card className="shadow-none">
      <CardHeader className="space-y-0 p-3">
        <div className="flex items-center gap-2">
          <button
            type="button"
            className="flex flex-1 items-center justify-between gap-3 rounded-md text-left"
            aria-expanded={expanded}
            onClick={() => setExpanded((value) => !value)}
          >
            <CardTitle className="text-sm">AI automation support</CardTitle>
            <ChevronDown className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform', expanded && 'rotate-180')} />
          </button>
          <HelpTooltip label="AI automation support help">AI tool used to support automation setup from workflow notes, CSVs, or existing material.</HelpTooltip>
        </div>
      </CardHeader>
      {expanded && (
        <CardContent className="space-y-3 pt-0">
          <Textarea
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
            placeholder="Describe the automation or workflow..."
            className="min-h-16 text-xs"
          />
          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" size="sm" variant="outline" disabled={!prompt.trim() && !material.trim()} onClick={generateDraft}>
              Suggest
            </Button>
            <Button type="button" size="sm" disabled={!draft} onClick={applyDraft}>
              Apply
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => setShowImport((value) => !value)}>
              {showImport ? 'Hide import' : 'Import file'}
            </Button>
          </div>
          {showImport && (
            <div className="rounded-lg border bg-muted/20 p-2">
              <FieldLabelWithHelp label="Material" help="Paste CSV headers, a checklist, or notes. Text files are read locally before suggestions are generated." className="text-xs" />
              <Textarea
                value={material}
                onChange={(event) => setMaterial(event.target.value)}
                placeholder="customer,status,next_action,due_date"
                className="mt-1 min-h-16 text-xs"
              />
              <Input
                type="file"
                accept=".csv,.txt,.md,text/csv,text/plain,image/*"
                className="mt-2 h-8 text-xs"
                onChange={(event) => void handleMaterialFile(event.target.files?.[0])}
              />
              {materialNote && <p className="mt-1 text-xs text-muted-foreground">{materialNote}</p>}
            </div>
          )}
          {draft && (
            <div className="space-y-2 rounded-lg border bg-muted/20 p-3 text-xs">
              <p className="font-medium">{draft.reply}</p>
              <div className="flex flex-wrap gap-1">
                {draft.fields.map(([label, type]) => (
                  <Badge key={`${label}:${type}`} variant="secondary">{label}</Badge>
                ))}
              </div>
              <p className="text-muted-foreground">Screens: {draft.screens.join(', ')}</p>
            </div>
          )}
        </CardContent>
      )}
    </Card>
  )
}

function validateWizardStep(form: DocType, step: WizardStep): string | null {
  if (step === 'Basics') {
    if (!form.name.trim()) return 'Name is required.'
    if (!form.module.trim()) return 'Area is required.'
  }
  if (step === 'Fields') {
    const fields = form.fields
      .map((field, index) => slugField(field.fieldname || field.label || `field_${index + 1}`))
      .filter(Boolean)
    if (fields.length === 0) return 'Add at least one field.'
    if (new Set(fields).size !== fields.length) return 'Field keys must be unique after system names are reserved.'
  }
  return null
}

function formatPreviewBlockers(blocked: Array<{ message: string; change: string }>): string {
  const top = blocked.slice(0, 3).map((entry) => entry.message || entry.change)
  return top.join(' ')
}

function PreflightSummary({ preview }: { preview: Awaited<ReturnType<typeof dryRunDoctype>> }) {
  const hasWarnings = preview.warnings.length > 0
  const hasBlocked = preview.blocked.length > 0
  const tone = hasBlocked ? 'border-destructive/50 bg-destructive/10 text-destructive' : hasWarnings ? 'border-amber-500/40 bg-amber-50 text-amber-900' : 'border-green-500/40 bg-green-50 text-green-900'

  return (
    <div className={cn('rounded-lg border p-3 text-sm', tone)}>
      <p className="font-medium">
        {hasBlocked ? 'Activation needs attention' : hasWarnings ? 'Activation looks mostly good' : 'Ready to activate'}
      </p>
      <p className="mt-1 text-sm">
        {hasBlocked
          ? 'Fix the blocked items below before saving and activating.'
          : hasWarnings
            ? 'Warnings are safe to review, but they may affect existing records.'
            : 'No blocking issues were found in the current configuration.'}
      </p>
      {(hasBlocked || hasWarnings) && (
        <ul className="mt-2 space-y-1 text-sm">
          {[...preview.blocked, ...preview.warnings].slice(0, 4).map((entry) => (
            <li key={`${entry.doctype}:${entry.change}:${entry.field ?? ''}`}>- {entry.message}</li>
          ))}
        </ul>
      )}
    </div>
  )
}

function inferAssistantDraft(prompt: string, form: DocType): AssistantDraft {
  const text = prompt.toLowerCase()
  if (text.includes('inventory') || text.includes('stock') || text.includes('warehouse')) {
    return {
      reply: 'I would model this as an inventory item with stock and reorder visibility.',
      fields: [['Item name', 'Data'], ['SKU', 'Data'], ['Category', 'Select'], ['Stock quantity', 'Int'], ['Reorder level', 'Int'], ['Selling price', 'Currency']],
      screens: ['table', 'cards', 'form'],
    }
  }
  if (text.includes('order') || text.includes('invoice') || text.includes('payment')) {
    return {
      reply: 'I would start with an order workflow and keep amount, dates, customer, and status visible.',
      fields: [['Customer', 'Link'], ['Order date', 'Date'], ['Due date', 'Date'], ['Status', 'Select'], ['Total', 'Currency'], ['Paid', 'Check']],
      screens: ['table', 'detail', 'form'],
    }
  }
  if (text.includes('visit') || text.includes('appointment') || text.includes('field')) {
    return {
      reply: 'I would track who was visited, the assigned owner, outcome, next action, and date.',
      fields: [['Customer', 'Link'], ['Assigned to', 'Link'], ['Visit date', 'Date'], ['Outcome', 'Select'], ['Next action', 'Text'], ['Next action date', 'Date']],
      screens: ['table', 'cards', 'form', 'detail'],
    }
  }
  if (text.includes('task') || text.includes('ticket') || text.includes('issue')) {
    return {
      reply: 'I would use a task-style tracker with owner, priority, due date, and completion status.',
      fields: [['Subject', 'Data'], ['Owner', 'Link'], ['Priority', 'Select'], ['Due date', 'Date'], ['Status', 'Select'], ['Done', 'Check']],
      screens: ['table', 'cards', 'form'],
    }
  }
  return {
    reply: form.name ? `I drafted a general ${form.name} data object with common operational fields.` : 'I drafted a general operational data object.',
    fields: [['Title', 'Data'], ['Owner', 'Link'], ['Status', 'Select'], ['Date', 'Date'], ['Amount', 'Currency'], ['Notes', 'Text']],
    screens: ['table', 'form'],
  }
}

function inferMaterialDraft(material: string, form: DocType): AssistantDraft {
  const fields = extractMaterialFields(material)
  return {
    reply: fields.length
      ? `I found ${fields.length} fields from your material. Review names and types before creating.`
      : 'I could not confidently extract fields, so I drafted a generic record structure.',
    fields: fields.length ? fields : [['Title', 'Data'], ['Status', 'Select'], ['Date', 'Date'], ['Notes', 'Text']],
    screens: chooseScreensForFields(fields, form),
  }
}

function extractMaterialFields(material: string): Array<[string, Field['fieldtype']]> {
  const firstMeaningfulLine = material
    .split(/\r?\n/)
    .map((line) => line.trim())
    .find((line) => line && !line.startsWith('#')) || ''
  const csvLike = firstMeaningfulLine.includes(',') || firstMeaningfulLine.includes('\t') || firstMeaningfulLine.includes(';')
  const rawNames = csvLike
    ? firstMeaningfulLine.split(/,|\t|;/)
    : material.split(/\r?\n/).flatMap((line) => line.split(/:|-/).slice(0, 1))

  return rawNames
    .map((name) => name.trim().replace(/^["']|["']$/g, ''))
    .filter((name) => name.length > 1)
    .slice(0, 14)
    .map((name) => [titleCase(name.replace(/_/g, ' ')), inferFieldType(name)])
}

function inferFieldType(name: string): Field['fieldtype'] {
  const normalized = name.toLowerCase()
  if (normalized.includes('date') || normalized.includes('deadline') || normalized.includes('due')) return 'Date'
  if (normalized.includes('time')) return 'Datetime'
  if (normalized.includes('amount') || normalized.includes('price') || normalized.includes('cost') || normalized.includes('total')) return 'Currency'
  if (normalized.includes('qty') || normalized.includes('quantity') || normalized.includes('count') || normalized.includes('stock')) return 'Int'
  if (normalized.includes('status') || normalized.includes('stage') || normalized.includes('priority') || normalized.includes('type')) return 'Select'
  if (normalized.includes('customer') || normalized.includes('owner') || normalized.includes('user') || normalized.includes('supplier')) return 'Link'
  if (normalized.includes('note') || normalized.includes('description') || normalized.includes('comment')) return 'Text'
  if (normalized.includes('audio') || normalized.includes('voice') || normalized.includes('recording') || normalized.includes('sound')) return 'Attach Audio'
  if (normalized.includes('photo') || normalized.includes('image') || normalized.includes('picture') || normalized.includes('avatar') || normalized.includes('logo')) return 'Attach Image'
  if (normalized.includes('file') || normalized.includes('attachment') || normalized.includes('document') || normalized.includes('invoice') || normalized.includes('pdf')) return 'Attach'
  if (normalized.startsWith('is_') || normalized.includes('done') || normalized.includes('paid')) return 'Check'
  return 'Data'
}

function chooseScreensForFields(fields: Array<[string, Field['fieldtype']]>, form: DocType): StandardPageKind[] {
  const hasMoney = fields.some(([, type]) => type === 'Currency')
  const hasManyFields = fields.length > 6
  if (form.is_submittable || hasMoney) return ['table', 'detail', 'form']
  if (hasManyFields) return ['table', 'form']
  return ['table', 'cards', 'form']
}

function inferObjectName(prompt: string): string {
  const text = prompt.toLowerCase()
  if (text.includes('inventory') || text.includes('stock')) return 'Inventory Item'
  if (text.includes('invoice')) return 'Invoice'
  if (text.includes('order')) return 'Order'
  if (text.includes('visit')) return 'Customer Visit'
  if (text.includes('ticket') || text.includes('issue')) return 'Support Ticket'
  if (text.includes('task')) return 'Task'
  return 'Workspace Record'
}

function inferModule(prompt: string): string {
  const text = prompt.toLowerCase()
  if (text.includes('inventory') || text.includes('stock') || text.includes('warehouse')) return 'Inventory'
  if (text.includes('invoice') || text.includes('payment')) return 'Finance'
  if (text.includes('order')) return 'Sales'
  if (text.includes('visit') || text.includes('field')) return 'Field Service'
  if (text.includes('ticket') || text.includes('issue')) return 'Support'
  return 'Workspace'
}

function normalizeWizardDoctype(form: DocType): DocType {
  const fields = form.fields
    .map((field, index) => ({
      ...field,
      fieldname: slugField(field.fieldname || field.label || `field_${index + 1}`),
      label: field.label || titleCase(field.fieldname || `Field ${index + 1}`),
      options: field.fieldtype === 'Select' ? normalizeSelectOptions(field.options) : field.options,
      in_list_view: index < 5 || field.in_list_view,
      search_index: index < 3 || field.search_index,
    }))
    .filter((field) => field.fieldname)

  return {
    ...form,
    name: titleCase(form.name.trim()),
    module: titleCase(form.module.trim()),
    title_field: fields[0]?.fieldname || 'title',
    search_fields: fields.slice(0, 4).map((field) => field.fieldname).join(','),
    track_changes: form.track_changes,
    fields,
  }
}

function applyStarterTemplate(form: DocType, fields: Array<[string, Field['fieldtype']]>): DocType {
  const starterFields = fields.map(([label, fieldtype], index) => createWizardField(label, fieldtype, index < 3))
  return {
    ...form,
    fields: starterFields,
  }
}

function updateWizardField(form: DocType, index: number, patch: Partial<Field>): DocType {
  const fields = [...form.fields]
  fields[index] = { ...fields[index], ...patch }
  return { ...form, fields }
}

function createWizardField(label: string, fieldtype: Field['fieldtype'] = 'Data', visible = true): Field {
  return {
    ...EMPTY_FIELD,
    label,
    fieldname: slugField(label),
    fieldtype,
    options: fieldtype === 'Select' ? 'Open\nClosed' : '',
    in_list_view: visible,
    search_index: visible,
  }
}

function toggleScreen(current: StandardPageKind[], screen: StandardPageKind): StandardPageKind[] {
  return current.includes(screen) ? current.filter((entry) => entry !== screen) : [...current, screen]
}


function normalizeSelectOptions(value: string): string {
  return value.includes('\n') ? value : value.split(',').map((entry) => entry.trim()).filter(Boolean).join('\n')
}

function friendlyFieldType(type: string): string {
  if (type === 'Data') return 'Short text'
  if (type === 'Text') return 'Long text'
  if (type === 'Int') return 'Number'
  if (type === 'Currency') return 'Money'
  if (type === 'Check') return 'Yes / no'
  if (type === 'Datetime') return 'Date & time'
  if (type === 'Select') return 'Choices'
  if (type === 'Link') return 'Link to data'
  if (type === 'Attach') return 'File'
  if (type === 'Attach Image') return 'Image'
  if (type === 'Attach Audio') return 'Audio'
  return type
}

function FieldRow({
  field,
  index,
  expanded,
  onToggle,
  onChange,
  onRemove,
  onMoveUp,
  onMoveDown,
  canMoveUp,
  canMoveDown,
  allDoctypes,
}: {
  field: Field
  index: number
  expanded: boolean
  onToggle: () => void
  onChange: (updates: Partial<Field>) => void
  onRemove: () => void
  onMoveUp: () => void
  onMoveDown: () => void
  canMoveUp: boolean
  canMoveDown: boolean
  allDoctypes: string[]
}) {
  const isLayout = ['Section Break', 'Column Break', 'Heading'].includes(field.fieldtype)
  const typeColor = isLayout ? 'bg-purple-100 text-purple-800' : 'bg-blue-100 text-blue-800'

  return (
    <div className="border rounded-lg">
      {/* Collapsed summary */}
      <div
        className="flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-muted/30"
        onClick={onToggle}
      >
        <button
          className="text-muted-foreground hover:text-foreground"
          onClick={(e) => { e.stopPropagation(); onToggle() }}
        >
          {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
        </button>
        <GripVertical className="h-4 w-4 text-muted-foreground/50 hidden sm:block" />
        <span className="text-sm font-medium min-w-0 sm:min-w-[100px] truncate">{field.fieldname || '(new)'}</span>
        <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${typeColor}`}>
          {field.fieldtype}
        </span>
        {field.options && !isLayout && (
          <span className="text-xs text-muted-foreground hidden sm:inline truncate max-w-[100px]">→ {field.options}</span>
        )}
        <span className="text-xs text-muted-foreground flex-1 hidden sm:inline">{field.label}</span>
        {field.reqd && (
          <span className="hidden sm:inline-flex items-center rounded-full bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400 px-1.5 py-0.5 text-[10px] font-bold">
            REQD
          </span>
        )}
        {field.in_list_view && (
          <span className="hidden sm:inline-flex items-center rounded-full bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400 px-1.5 py-0.5 text-[10px] font-medium">
            LIST
          </span>
        )}
        <div className="ml-auto flex items-center gap-0.5 shrink-0" onClick={(e) => e.stopPropagation()}>
          <Button variant="ghost" size="icon" className="h-6 w-6 hidden sm:inline-flex" aria-label="Move field up" title="Move field up" onClick={() => onMoveUp()} disabled={!canMoveUp}>
            <ChevronRight className="h-3 w-3 rotate-[-90deg]" />
          </Button>
          <Button variant="ghost" size="icon" className="h-6 w-6 hidden sm:inline-flex" aria-label="Move field down" title="Move field down" onClick={() => onMoveDown()} disabled={!canMoveDown}>
            <ChevronRight className="h-3 w-3 rotate-90" />
          </Button>
          <Button variant="ghost" size="icon" className="h-6 w-6" aria-label="Toggle field details" title="Toggle details" onClick={onToggle}>
            <Edit className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-6 w-6" aria-label="Remove field" title="Remove field" onClick={onRemove}>
            <Trash2 className="h-3 w-3 text-destructive" />
          </Button>
        </div>
      </div>

      {/* Expanded editor */}
      {expanded && (
        <div className="px-4 pb-4 border-t">
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mt-3">
            <div>
              <Label>Fieldname *</Label>
              <Input
                value={field.fieldname}
                onChange={(e) => onChange({ fieldname: e.target.value })}
                placeholder="field_name"
              />
            </div>
            <div>
              <Label>Label</Label>
              <Input
                value={field.label}
                onChange={(e) => onChange({ label: e.target.value })}
              />
            </div>
            <div>
              <Label>Type *</Label>
              <Select value={field.fieldtype} onValueChange={(v) => onChange({ fieldtype: v as any })}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {FIELD_TYPE_GROUPS.map((group) => (
                    <div key={group.label}>
                      <div className="px-2 py-1 text-xs font-semibold text-muted-foreground">{group.label}</div>
                      {group.types.map((t) => (
                        <SelectItem key={t} value={t}>{t}</SelectItem>
                      ))}
                    </div>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {/* Type-specific options */}
          {field.fieldtype === 'Select' && (
            <div className="mt-3">
              <Label>Options (one per line)</Label>
              <textarea
                className="w-full mt-1 rounded-md border bg-background px-3 py-2 text-sm font-mono min-h-[80px]"
                value={field.options}
                onChange={(e) => onChange({ options: e.target.value })}
                placeholder="Option 1\nOption 2\nOption 3"
              />
            </div>
          )}
          {(field.fieldtype === 'Link' || field.fieldtype === 'Dynamic Link') && (
            <div className="mt-3">
              <Label>Target DocType</Label>
              <Select value={field.options} onValueChange={(v) => onChange({ options: v || '' })}>
                <SelectTrigger className="mt-1">
                  <SelectValue placeholder="Select target doctype..." />
                </SelectTrigger>
                <SelectContent>
                  {allDoctypes.map((name) => (
                    <SelectItem key={name} value={name}>{name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          {field.fieldtype === 'Table' && (
            <div className="mt-3">
              <Label>Child DocType Name</Label>
              <Input
                className="mt-1"
                value={field.options}
                onChange={(e) => onChange({ options: e.target.value })}
                placeholder="Order Item"
              />
            </div>
          )}

          {/* Display options */}
          <div className="grid grid-cols-2 sm:flex sm:flex-wrap gap-2 sm:gap-4 mt-4">
            <label className="flex items-center gap-1.5 text-sm">
              <Switch checked={field.reqd} onCheckedChange={(v) => onChange({ reqd: v })} /> Required
            </label>
            <label className="flex items-center gap-1.5 text-sm">
              <Switch checked={field.unique} onCheckedChange={(v) => onChange({ unique: v })} /> Unique
            </label>
            <label className="flex items-center gap-1.5 text-sm">
              <Switch checked={field.read_only} onCheckedChange={(v) => onChange({ read_only: v })} /> Read Only
            </label>
            <label className="flex items-center gap-1.5 text-sm">
              <Switch checked={field.bold} onCheckedChange={(v) => onChange({ bold: v })} /> Bold
            </label>
            <label className="flex items-center gap-1.5 text-sm">
              <Switch checked={field.hidden} onCheckedChange={(v) => onChange({ hidden: v })} /> Hidden
            </label>
            <label className="flex items-center gap-1.5 text-sm">
              <Switch checked={field.in_list_view} onCheckedChange={(v) => onChange({ in_list_view: v })} /> In List View
            </label>
            <label className="flex items-center gap-1.5 text-sm">
              <Switch checked={field.in_standard_filter} onCheckedChange={(v) => onChange({ in_standard_filter: v })} /> In Filter
            </label>
            <label className="flex items-center gap-1.5 text-sm">
              <Switch checked={field.search_index} onCheckedChange={(v) => onChange({ search_index: v })} /> Search Index
            </label>
          </div>

          {/* Default & Description */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4 mt-4">
            <div>
              <Label>Default Value</Label>
              <Input
                value={field.default}
                onChange={(e) => onChange({ default: e.target.value })}
              />
            </div>
            <div>
              <Label>Description</Label>
              <Input
                value={field.description}
                onChange={(e) => onChange({ description: e.target.value })}
              />
            </div>
          </div>

          {/* Linked Field */}
          {!isLayout && (
            <div className="mt-3">
              <Label>Linked Field (auto-populate from linked document)</Label>
              <Input
                className="mt-1"
                value={field.linked_field || ''}
                onChange={(e) => onChange({ linked_field: e.target.value })}
                placeholder="product.selling_price"
              />
            </div>
          )}

          {/* Computed */}
          {!isLayout && (
            <div className="mt-3">
              <Label>Computed Expression</Label>
              <LispAutocomplete
                className="mt-1 font-mono text-sm"
                value={field.computed || ''}
                onChange={(val) => onChange({ computed: val })}
                placeholder={field.computed?.startsWith('(') ? '(sum "items" "amount")' : 'quantity * unit_price'}
              />
            </div>
          )}

          {/* Constraints */}
          {!isLayout && (
            <div className="mt-3 border-t pt-3">
              <div className="flex items-center justify-between mb-2">
                <Label>Constraints</Label>
                <Button
                  variant="ghost" size="sm"
                  onClick={() => {
                    const updated = [...(field.constraints || []), { type: 'max', value: undefined as any, message: '' }]
                    onChange({ constraints: updated })
                  }}
                >+ Add</Button>
              </div>
              {(field.constraints || []).map((c, ci) => (
                <div key={ci} className="grid grid-cols-12 gap-2 mb-2 items-start">
                  <select
                    className="col-span-3 h-9 rounded-md border bg-background px-2 text-sm"
                    value={c.type}
                    onChange={(e) => {
                      const updated = [...(field.constraints || [])]
                      updated[ci] = { ...updated[ci], type: e.target.value }
                      onChange({ constraints: updated })
                    }}
                  >
                    <option value="max">max</option>
                    <option value="min">min</option>
                    <option value="max_length">max_length</option>
                    <option value="min_length">min_length</option>
                    <option value="max_rows">max_rows</option>
                    <option value="min_rows">min_rows</option>
                    <option value="regex">regex</option>
                    <option value="one_of">one_of</option>
                    <option value="not_one_of">not_one_of</option>
                    <option value="min_date">min_date</option>
                    <option value="max_date">max_date</option>
                  </select>
                  <Input
                    className="col-span-3 h-9 text-sm"
                    value={c.value != null ? String(c.value) : ''}
                    onChange={(e) => {
                      const v = e.target.value
                      const num = Number(v)
                      const updated = [...(field.constraints || [])]
                      updated[ci] = { ...updated[ci], value: isNaN(num) ? v : num }
                      onChange({ constraints: updated })
                    }}
                    placeholder="value"
                  />
                  <Input
                    className="col-span-5 h-9 text-sm"
                    value={c.message || ''}
                    onChange={(e) => {
                      const updated = [...(field.constraints || [])]
                      updated[ci] = { ...updated[ci], message: e.target.value }
                      onChange({ constraints: updated })
                    }}
                    placeholder="Error message"
                  />
                  <Button
                    variant="ghost" size="sm"
                    className="col-span-1 h-9 text-destructive"
                    onClick={() => {
                      const updated = (field.constraints || []).filter((_, i) => i !== ci)
                      onChange({ constraints: updated.length > 0 ? updated : null })
                    }}
                  >✕</Button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

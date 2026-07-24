import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchDoctypeSchema, fetchDoctypes } from '@/lib/api/system'
import { useBuilderStore } from '@/lib/builder-store'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ChevronDown, ChevronRight, Copy, Plus, Trash2 } from 'lucide-react'
import type { ViewAction, ViewComponent, ViewFilter, ViewRule } from '@/lib/api/views'
import type { Field } from '@/types/kora'

const REGIONS = ['main', 'side', 'left', 'right', 'header', 'footer']
const ACTION_TRIGGERS = ['on_click', 'on_submit', 'on_scan', 'on_drag', 'on_change']
const ACTION_TYPES = [
  'navigate',
  'create_record',
  'update_record',
  'delete_record',
  'workflow_transition',
  'local_cart_add',
  'local_cart_remove',
  'local_state_set',
  'call_script',
  'call_webhook',
  'create_transaction',
]
const FILTER_OPS = ['equals', 'not_equals', 'in', 'like', 'gt', 'gte', 'lt', 'lte']
const RULE_OPS = ['equals', 'not_equals', 'gt', 'gte', 'lt', 'lte', 'in', 'is_set', 'is_not_set']
const RULE_TARGETS = ['visible', 'hidden', 'disabled', 'readonly']
const BINDING_PRESETS: Record<string, string[]> = {
  product_grid: ['title', 'subtitle', 'price', 'badge', 'image'],
  category_tabs: ['group_field', 'filter_key'],
  payment_panel: ['methods'],
  metric_card: ['view', 'fn', 'field', 'trend', 'title'],
  recent_records: ['title', 'date', 'limit'],
  scanner_input: ['search_fields'],
  record_table: ['title', 'status'],
  record_list: ['title', 'subtitle', 'status'],
  record_cards: ['title', 'subtitle', 'image'],
  calendar_view: ['title', 'date', 'color'],
  kanban_board: ['title', 'status', 'subtitle'],
}
const NON_FIELD_BINDINGS = new Set(['methods', 'fn', 'limit', 'view', 'trend', 'search_fields', 'filter_key'])
const CONTAINER_TYPES = new Set(['dashboard_grid', 'split_view', 'tabs', 'wizard', 'drawer', 'print_layout', 'workspace_dashboard'])

function findComponent(comps: ViewComponent[], id: string): ViewComponent | null {
  for (const c of comps) {
    if (c.id === id) return c
    if (c.components?.length) {
      const found = findComponent(c.components, id)
      if (found) return found
    }
  }
  return null
}

function fieldOptions(schema: any): Field[] {
  return schema?.doctype?.fields?.filter((f: Field) => !['Table', 'Section Break', 'Column Break', 'Heading'].includes(f.fieldtype)) || []
}

function fieldByName(fields: Field[], name: string): Field | undefined {
  return fields.find((f) => f.fieldname === name)
}

function selectValue(value?: string) {
  return value && value.trim() ? value : '__none'
}

function fromSelectValue(value: string | null | undefined) {
  return value === '__none' || value == null ? '' : value
}

function parseValueForField(raw: string, field?: Field) {
  if (!field) return raw
  if (['Int', 'Float', 'Currency', 'Percent'].includes(field.fieldtype)) return raw === '' ? '' : Number(raw)
  if (field.fieldtype === 'Check') return raw === 'true'
  if (field.fieldtype === 'JSON') {
    try { return JSON.parse(raw) } catch { return raw }
  }
  return raw
}

function formatRuleValue(value: any) {
  if (Array.isArray(value)) return value.join(', ')
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  return String(value ?? '')
}

export function Inspector() {
  const working = useBuilderStore((s) => s.working)
  const selectedId = useBuilderStore((s) => s.selectedComponentId)
  const updateComponent = useBuilderStore((s) => s.updateComponent)
  const addChildComponent = useBuilderStore((s) => s.addChildComponent)
  const removeComponent = useBuilderStore((s) => s.removeComponent)
  const selectComponent = useBuilderStore((s) => s.selectComponent)

  if (!working?.view) {
    return <InspectorShell><p className="text-sm text-muted-foreground text-center">Loading...</p></InspectorShell>
  }
  if (!selectedId) {
    return <InspectorShell><p className="text-sm text-muted-foreground text-center">Select a component on the canvas to edit its properties</p></InspectorShell>
  }

  const comp = findComponent(working.view.components || [], selectedId)
  if (!comp) {
    return <InspectorShell><p className="text-sm text-muted-foreground">Component not found</p></InspectorShell>
  }

  return (
    <div className="h-full border-l bg-muted/10 overflow-y-auto">
      <div className="flex items-center justify-between gap-2 border-b bg-muted/20 p-3">
        <div className="min-w-0">
          <h3 className="truncate text-xs font-semibold uppercase tracking-wide text-muted-foreground">{comp.type.replace(/_/g, ' ')}</h3>
          <p className="truncate text-[11px] text-muted-foreground font-mono">{comp.id}</p>
        </div>
        <Button variant="ghost" size="icon" className="h-7 w-7" title="Copy component JSON" onClick={() => navigator.clipboard?.writeText(JSON.stringify(comp, null, 2))}>
          <Copy className="h-3.5 w-3.5" />
        </Button>
      </div>
      <div className="p-3 space-y-3">
        <BasicSection comp={comp} update={(p) => updateComponent(selectedId, p)} />
        <DataSection comp={comp} update={(p) => updateComponent(selectedId, p)} />
        <DisplaySection comp={comp} update={(p) => updateComponent(selectedId, p)} />
        <ActionsSection comp={comp} update={(p) => updateComponent(selectedId, p)} />
        <FiltersSection comp={comp} update={(p) => updateComponent(selectedId, p)} />
        <RulesSection comp={comp} update={(p) => updateComponent(selectedId, p)} />
        {CONTAINER_TYPES.has(comp.type) && (
          <ChildrenSection
            comp={comp}
            onAdd={(type) => addChildComponent(selectedId, type)}
            onSelect={selectComponent}
            onRemove={removeComponent}
          />
        )}
        <RawJSONSection comp={comp} update={(p) => updateComponent(selectedId, p)} />
      </div>
    </div>
  )
}

function InspectorShell({ children }: { children: React.ReactNode }) {
  return <div className="h-full flex items-center justify-center border-l bg-muted/10 p-4">{children}</div>
}

function Section({ title, defaultOpen = true, children }: { title: string; defaultOpen?: boolean; children: React.ReactNode }) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div className="rounded-md border bg-card">
      <button className="flex items-center gap-1 w-full px-3 py-2 text-xs font-semibold hover:bg-muted/30" onClick={() => setOpen(!open)}>
        {open ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
        {title}
      </button>
      {open && <div className="px-3 pb-3 space-y-3">{children}</div>}
    </div>
  )
}

function BasicSection({ comp, update }: { comp: ViewComponent; update: (p: Partial<ViewComponent>) => void }) {
  return (
    <Section title="Basic">
      <TextField label="ID" value={comp.id} onChange={(id) => update({ id })} mono />
      <TextField label="Label" value={comp.label || ''} onChange={(label) => update({ label })} placeholder={comp.type.replace(/_/g, ' ')} />
      <div className="space-y-1.5">
        <Label className="text-[11px]">Region</Label>
        <Select value={selectValue(comp.region)} onValueChange={(v) => update({ region: fromSelectValue(v as string) || 'main' })}>
          <SelectTrigger className="h-7 text-xs"><SelectValue /></SelectTrigger>
          <SelectContent>{REGIONS.map((r) => <SelectItem key={r} value={r}>{r}</SelectItem>)}</SelectContent>
        </Select>
      </div>
    </Section>
  )
}

function DataSection({ comp, update }: { comp: ViewComponent; update: (p: Partial<ViewComponent>) => void }) {
  const { data: doctypes } = useQuery({ queryKey: ['doctypes'], queryFn: fetchDoctypes, staleTime: 5 * 60_000 })
  const { data: schema } = useQuery({
    queryKey: ['doctype', comp.source_doctype],
    queryFn: () => fetchDoctypeSchema(comp.source_doctype || ''),
    enabled: !!comp.source_doctype,
    staleTime: 5 * 60_000,
  })
  const fields = fieldOptions(schema)
  const bindings = comp.bindings || {}
  const bindingEntries = Object.entries(bindings)
  const presets = BINDING_PRESETS[comp.type] || ['title', 'subtitle', 'status']

  const updateBinding = (prop: string, value: string) => update({ bindings: { ...bindings, [prop]: value } })
  const removeBinding = (prop: string) => {
    const next = { ...bindings }
    delete next[prop]
    update({ bindings: next })
  }

  return (
    <Section title="Data Source">
      <div className="space-y-1.5">
        <Label className="text-[11px]">Source DocType</Label>
        <Select value={selectValue(comp.source_doctype)} onValueChange={(v) => update({ source_doctype: fromSelectValue(v as string), bindings: comp.bindings || {} })}>
          <SelectTrigger className="h-7 text-xs"><SelectValue placeholder="Choose doctype" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="__none">None</SelectItem>
            {(doctypes || []).map((dt) => <SelectItem key={dt.name} value={dt.name}>{dt.name}</SelectItem>)}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <Label className="text-[11px]">Bindings</Label>
          <Select onValueChange={(prop) => { const key = String(prop || ''); if (key) updateBinding(key, bindings[key] || '') }}>
            <SelectTrigger className="h-6 w-24 text-[10px]"><SelectValue placeholder="Add" /></SelectTrigger>
            <SelectContent>{presets.map((p) => <SelectItem key={p} value={p}>{p}</SelectItem>)}</SelectContent>
          </Select>
        </div>
        {bindingEntries.length === 0 && <p className="text-xs text-muted-foreground">No bindings</p>}
        {bindingEntries.map(([prop, value]) => (
          <div key={prop} className="grid grid-cols-[1fr_1.2fr_auto] items-center gap-1">
            <Input
              value={prop}
              onChange={(e) => {
                const next = { ...bindings }
                delete next[prop]
                next[e.target.value] = value
                update({ bindings: next })
              }}
              placeholder="prop"
              className="h-7 text-xs"
            />
            {NON_FIELD_BINDINGS.has(prop) ? (
              <Input value={value || ''} onChange={(e) => updateBinding(prop, e.target.value)} placeholder="value" className="h-7 text-xs" />
            ) : (
              <FieldSelect fields={fields} value={value} onChange={(field) => updateBinding(prop, field)} />
            )}
            <IconButton label="Remove binding" onClick={() => removeBinding(prop)}><Trash2 className="h-3 w-3 text-destructive" /></IconButton>
          </div>
        ))}
      </div>
    </Section>
  )
}

function DisplaySection({ comp, update }: { comp: ViewComponent; update: (p: Partial<ViewComponent>) => void }) {
  const { data: schema } = useQuery({
    queryKey: ['doctype', comp.source_doctype],
    queryFn: () => fetchDoctypeSchema(comp.source_doctype || ''),
    enabled: !!comp.source_doctype,
    staleTime: 5 * 60_000,
  })
  const fields = fieldOptions(schema)

  return (
    <Section title="Display">
      <ColumnPicker label="Desktop Columns" fields={fields} selected={comp.desktop_columns || []} onChange={(desktop_columns) => update({ desktop_columns })} />
      <ColumnPicker label="Mobile Columns" fields={fields} selected={comp.mobile_columns || []} max={3} onChange={(mobile_columns) => update({ mobile_columns })} />
      <div className="space-y-1.5">
        <Label className="text-[11px]">Grid Span</Label>
        <Input type="number" min={1} max={12} value={comp.span || ''} onChange={(e) => update({ span: parseInt(e.target.value) || 0 })} className="h-7 text-xs w-24" />
      </div>
    </Section>
  )
}

function ActionsSection({ comp, update }: { comp: ViewComponent; update: (p: Partial<ViewComponent>) => void }) {
  const actions = comp.actions || []
  const addAction = () => update({ actions: [...actions, { id: `action_${Date.now()}`, trigger: 'on_click', type: 'navigate', config: {} }] })
  const removeAction = (id: string) => update({ actions: actions.filter((a) => a.id !== id) })
  const updateAction = (id: string, patch: Partial<ViewAction>) => update({ actions: actions.map((a) => a.id === id ? { ...a, ...patch } : a) })

  return (
    <Section title="Actions">
      {actions.length === 0 ? <p className="text-xs text-muted-foreground">No actions</p> : actions.map((action) => (
        <div key={action.id} className="space-y-2 rounded-md border bg-muted/10 p-2">
          <div className="grid grid-cols-[1fr_auto] gap-1">
            <TextField label="Action ID" value={action.id} onChange={(id) => updateAction(action.id, { id })} mono />
            <div className="pt-5"><IconButton label="Remove action" onClick={() => removeAction(action.id)}><Trash2 className="h-3 w-3 text-destructive" /></IconButton></div>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <OptionField label="Trigger" value={action.trigger} options={ACTION_TRIGGERS} onChange={(trigger) => updateAction(action.id, { trigger })} />
            <OptionField label="Type" value={action.type} options={ACTION_TYPES} onChange={(type) => updateAction(action.id, { type, config: defaultActionConfig(type, action.config) })} />
          </div>
          <ActionConfigEditor action={action} update={(config) => updateAction(action.id, { config })} sourceDoctype={comp.source_doctype} />
        </div>
      ))}
      <Button variant="outline" size="sm" className="w-full h-7 text-xs" onClick={addAction}><Plus className="h-3 w-3 mr-1" />Add Action</Button>
    </Section>
  )
}

function ActionConfigEditor({ action, update, sourceDoctype }: { action: ViewAction; update: (config: Record<string, any>) => void; sourceDoctype?: string }) {
  const config = action.config || {}
  const { data: doctypes } = useQuery({ queryKey: ['doctypes'], queryFn: fetchDoctypes, staleTime: 5 * 60_000 })
  const target = String(config.target_doctype || config.doctype || sourceDoctype || '')
  const { data: targetSchema } = useQuery({ queryKey: ['doctype', target], queryFn: () => fetchDoctypeSchema(target), enabled: !!target, staleTime: 5 * 60_000 })
  const tableFields = targetSchema?.doctype?.fields?.filter((f: Field) => f.fieldtype === 'Table') || []
  const transitions = targetSchema?.workflow?.transitions?.map((t: any) => t.action) || []

  const set = (key: string, value: any) => update({ ...config, [key]: value })
  const setTarget = (value: string) => {
    const nextKey = action.type === 'workflow_transition' ? 'doctype' : 'target_doctype'
    update({ ...config, [nextKey]: value })
  }

  switch (action.type) {
    case 'navigate':
      return <TextField label="URL Template" value={String(config.to || '')} onChange={(to) => set('to', to)} placeholder="/workspace/Product/{name}" mono />
    case 'create_record':
    case 'update_record':
    case 'delete_record':
      return <DocTypeSelect label="Target DocType" value={String(config.target_doctype || '')} doctypes={doctypes || []} onChange={(value) => set('target_doctype', value)} />
    case 'workflow_transition':
      return <div className="grid grid-cols-2 gap-2"><DocTypeSelect label="DocType" value={String(config.doctype || '')} doctypes={doctypes || []} onChange={setTarget} /><OptionField label="Transition" value={String(config.transition || '')} options={transitions} onChange={(transition) => set('transition', transition)} /></div>
    case 'create_transaction':
      return <div className="space-y-2 rounded-md border bg-background p-2"><DocTypeSelect label="Target DocType" value={String(config.target_doctype || '')} doctypes={doctypes || []} onChange={setTarget} /><OptionField label="Child Table" value={String(config.child_table || '')} options={tableFields.flatMap((f: Field) => [f.fieldname, f.options].filter(Boolean))} onChange={(child_table) => set('child_table', child_table)} /><TextField label="Line Source" value={String(config.line_source || 'cart')} onChange={(line_source) => set('line_source', line_source)} /></div>
    case 'local_cart_add':
      return <div className="grid grid-cols-2 gap-2"><TextField label="Product Field" value={String(config.product_field || 'product')} onChange={(v) => set('product_field', v)} /><TextField label="Rate Field" value={String(config.rate_field || 'rate')} onChange={(v) => set('rate_field', v)} /></div>
    case 'local_cart_remove':
      return <TextField label="Item Key" value={String(config.item_key || 'name')} onChange={(item_key) => set('item_key', item_key)} />
    case 'local_state_set':
      return <div className="grid grid-cols-2 gap-2"><TextField label="State Key" value={String(config.key || '')} onChange={(key) => set('key', key)} /><TextField label="Value Source" value={String(config.value || '')} onChange={(value) => set('value', value)} /></div>
    case 'call_script':
      return <TextField label="Script Name" value={String(config.script || '')} onChange={(script) => set('script', script)} />
    case 'call_webhook':
      return <TextField label="Webhook Name" value={String(config.webhook || '')} onChange={(webhook) => set('webhook', webhook)} />
    default:
      return null
  }
}

function FiltersSection({ comp, update }: { comp: ViewComponent; update: (p: Partial<ViewComponent>) => void }) {
  const filters = comp.filters || []
  const { data: schema } = useQuery({ queryKey: ['doctype', comp.source_doctype], queryFn: () => fetchDoctypeSchema(comp.source_doctype || ''), enabled: !!comp.source_doctype, staleTime: 5 * 60_000 })
  const fields = fieldOptions(schema)
  const updateFilter = (index: number, patch: Partial<ViewFilter>) => update({ filters: filters.map((f, i) => i === index ? { ...f, ...patch } : f) })

  return (
    <Section title="Filters" defaultOpen={false}>
      {filters.length === 0 && <p className="text-xs text-muted-foreground">No filters</p>}
      {filters.map((filter, i) => (
        <div key={i} className="grid grid-cols-[1fr_0.8fr_1fr_auto] gap-1 items-end">
          <FieldSelect fields={fields} value={filter.field} onChange={(field) => updateFilter(i, { field })} />
          <OptionField value={filter.op} options={FILTER_OPS} onChange={(op) => updateFilter(i, { op })} />
          <Input className="h-7 text-xs" value={String(filter.value ?? '')} onChange={(e) => updateFilter(i, { value: parseValueForField(e.target.value, fieldByName(fields, filter.field)) })} />
          <IconButton label="Remove filter" onClick={() => update({ filters: filters.filter((_, j) => j !== i) })}><Trash2 className="h-3 w-3 text-destructive" /></IconButton>
        </div>
      ))}
      <Button variant="outline" size="sm" className="w-full h-7 text-xs" onClick={() => update({ filters: [...filters, { field: '', op: 'equals', value: '' }] })}><Plus className="h-3 w-3 mr-1" />Add Filter</Button>
    </Section>
  )
}

function RulesSection({ comp, update }: { comp: ViewComponent; update: (p: Partial<ViewComponent>) => void }) {
  const rules = comp.rules || []
  const { data: schema } = useQuery({ queryKey: ['doctype', comp.source_doctype], queryFn: () => fetchDoctypeSchema(comp.source_doctype || ''), enabled: !!comp.source_doctype, staleTime: 5 * 60_000 })
  const fields = fieldOptions(schema)
  const updateRule = (index: number, patch: Partial<ViewRule>) => update({ rules: rules.map((r, i) => i === index ? { ...r, ...patch } : r) })

  return (
    <Section title="Rules" defaultOpen={false}>
      {rules.length === 0 && <p className="text-xs text-muted-foreground">No rules</p>}
      {rules.map((rule, i) => {
        const field = fieldByName(fields, rule.condition.field)
        return (
          <div key={i} className="space-y-1 rounded-md border bg-muted/10 p-2">
            <div className="grid grid-cols-[0.9fr_1.1fr_auto] gap-1 items-end">
              <OptionField label="Target" value={rule.target} options={RULE_TARGETS} onChange={(target) => updateRule(i, { target })} />
              <FieldSelect label="Field" fields={fields} value={rule.condition.field} onChange={(fieldName) => updateRule(i, { condition: { ...rule.condition, field: fieldName } })} />
              <IconButton label="Remove rule" onClick={() => update({ rules: rules.filter((_, j) => j !== i) })}><Trash2 className="h-3 w-3 text-destructive" /></IconButton>
            </div>
            <div className="grid grid-cols-2 gap-1">
              <OptionField label="Operator" value={rule.condition.op} options={RULE_OPS} onChange={(op) => updateRule(i, { condition: { ...rule.condition, op } })} />
              <TypedValueField label="Value" field={field} value={rule.condition.value} onChange={(value) => updateRule(i, { condition: { ...rule.condition, value } })} />
            </div>
          </div>
        )
      })}
      <Button variant="outline" size="sm" className="w-full h-7 text-xs" onClick={() => update({ rules: [...rules, { target: 'hidden', condition: { field: '', op: 'equals', value: '' } }] })}><Plus className="h-3 w-3 mr-1" />Add Rule</Button>
    </Section>
  )
}

function ChildrenSection({ comp, onAdd, onSelect, onRemove }: { comp: ViewComponent; onAdd: (type: string) => void; onSelect: (id: string) => void; onRemove: (id: string) => void }) {
  return (
    <Section title={`Children (${comp.components?.length || 0})`} defaultOpen={false}>
      <div className="space-y-1">
        {(comp.components || []).map((child) => (
          <div key={child.id} className="flex items-center gap-2 rounded-md border bg-background px-2 py-1.5">
            <button className="min-w-0 flex-1 text-left" onClick={() => onSelect(child.id)}>
              <p className="truncate text-xs font-medium">{child.label || child.type.replace(/_/g, ' ')}</p>
              <p className="truncate text-[10px] text-muted-foreground font-mono">{child.type}</p>
            </button>
            <IconButton label="Remove child" onClick={() => onRemove(child.id)}><Trash2 className="h-3 w-3 text-destructive" /></IconButton>
          </div>
        ))}
      </div>
      <Select onValueChange={(type) => { const nextType = String(type || ''); if (nextType) onAdd(nextType) }}>
        <SelectTrigger className="h-7 text-xs"><SelectValue placeholder="Add child component" /></SelectTrigger>
        <SelectContent>{['metric_card', 'chart', 'record_table', 'record_list', 'record_detail', 'tabs', 'filter_bar'].map((type) => <SelectItem key={type} value={type}>{type}</SelectItem>)}</SelectContent>
      </Select>
    </Section>
  )
}

function RawJSONSection({ comp, update }: { comp: ViewComponent; update: (p: Partial<ViewComponent>) => void }) {
  const [text, setText] = useState(() => JSON.stringify(comp, null, 2))
  const [error, setError] = useState('')
  return (
    <Section title="Advanced JSON" defaultOpen={false}>
      <textarea className="h-44 w-full rounded-md border bg-background p-2 font-mono text-[11px]" value={text} onChange={(e) => setText(e.target.value)} />
      {error && <p className="text-xs text-destructive">{error}</p>}
      <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => {
        try {
          const parsed = JSON.parse(text)
          setError('')
          update(parsed)
        } catch (err: any) {
          setError(err.message || 'Invalid JSON')
        }
      }}>Apply JSON</Button>
    </Section>
  )
}

function ColumnPicker({ label, fields, selected, max, onChange }: { label: string; fields: Field[]; selected: string[]; max?: number; onChange: (value: string[]) => void }) {
  const toggle = (field: string) => {
    if (selected.includes(field)) return onChange(selected.filter((f) => f !== field))
    if (max && selected.length >= max) return onChange([...selected.slice(1), field])
    onChange([...selected, field])
  }
  return (
    <div className="space-y-1.5">
      <Label className="text-[11px]">{label}</Label>
      <div className="flex max-h-28 flex-wrap gap-1 overflow-auto rounded-md border bg-background p-1.5">
        {fields.length === 0 && <span className="text-xs text-muted-foreground">Choose a DocType first</span>}
        {fields.map((field) => (
          <button key={field.fieldname} type="button" className={`rounded-full border px-2 py-1 text-[11px] ${selected.includes(field.fieldname) ? 'bg-primary text-primary-foreground border-primary' : 'bg-muted/30 hover:bg-muted'}`} onClick={() => toggle(field.fieldname)}>
            {field.label || field.fieldname}
          </button>
        ))}
      </div>
    </div>
  )
}

function FieldSelect({ label, fields, value, onChange }: { label?: string; fields: Field[]; value?: string; onChange: (value: string) => void }) {
  return (
    <div className={label ? 'space-y-1.5' : ''}>
      {label && <Label className="text-[11px]">{label}</Label>}
      <Select value={selectValue(value)} onValueChange={(v) => onChange(fromSelectValue(v as string))}>
        <SelectTrigger className="h-7 text-xs"><SelectValue placeholder="Field" /></SelectTrigger>
        <SelectContent>
          <SelectItem value="__none">None</SelectItem>
          {fields.map((f) => <SelectItem key={f.fieldname} value={f.fieldname}>{f.label || f.fieldname}</SelectItem>)}
        </SelectContent>
      </Select>
    </div>
  )
}

function DocTypeSelect({ label, value, doctypes, onChange }: { label: string; value: string; doctypes: any[]; onChange: (value: string) => void }) {
  return (
    <div className="space-y-1.5">
      <Label className="text-[11px]">{label}</Label>
      <Select value={selectValue(value)} onValueChange={(v) => onChange(fromSelectValue(v as string))}>
        <SelectTrigger className="h-7 text-xs"><SelectValue placeholder="Choose doctype" /></SelectTrigger>
        <SelectContent><SelectItem value="__none">None</SelectItem>{doctypes.map((dt) => <SelectItem key={dt.name} value={dt.name}>{dt.name}</SelectItem>)}</SelectContent>
      </Select>
    </div>
  )
}

function OptionField({ label, value, options, onChange }: { label?: string; value?: string; options: string[]; onChange: (value: string) => void }) {
  return (
    <div className={label ? 'space-y-1.5' : ''}>
      {label && <Label className="text-[11px]">{label}</Label>}
      <Select value={selectValue(value)} onValueChange={(v) => onChange(fromSelectValue(v as string))}>
        <SelectTrigger className="h-7 text-xs"><SelectValue /></SelectTrigger>
        <SelectContent><SelectItem value="__none">None</SelectItem>{options.map((o) => <SelectItem key={o} value={o}>{o}</SelectItem>)}</SelectContent>
      </Select>
    </div>
  )
}

function TypedValueField({ label, field, value, onChange }: { label: string; field?: Field; value: any; onChange: (value: any) => void }) {
  if (field?.fieldtype === 'Check') {
    return <OptionField label={label} value={value ? 'true' : 'false'} options={['true', 'false']} onChange={(v) => onChange(v === 'true')} />
  }
  if (field?.fieldtype === 'Select') {
    const options = (field.options || '').split('\n').map((o) => o.trim()).filter(Boolean)
    return <OptionField label={label} value={String(value || '')} options={options} onChange={onChange} />
  }
  const inputType = field && ['Int', 'Float', 'Currency', 'Percent'].includes(field.fieldtype) ? 'number' : field && ['Date', 'Datetime', 'Time'].includes(field.fieldtype) ? (field.fieldtype === 'Datetime' ? 'datetime-local' : field.fieldtype.toLowerCase()) : 'text'
  return <TextField label={label} type={inputType} value={formatRuleValue(value)} onChange={(raw) => onChange(parseValueForField(raw, field))} />
}

function TextField({ label, value, onChange, placeholder, mono, type = 'text' }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string; mono?: boolean; type?: string }) {
  return <div className="space-y-1.5"><Label className="text-[11px]">{label}</Label><Input type={type} value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} className={`h-7 text-xs ${mono ? 'font-mono' : ''}`} /></div>
}

function IconButton({ label, onClick, children }: { label: string; onClick: () => void; children: React.ReactNode }) {
  return <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" title={label} onClick={onClick}>{children}</Button>
}

function defaultActionConfig(type: string, existing?: Record<string, any>) {
  const config = existing || {}
  switch (type) {
    case 'navigate': return { to: config.to || '' }
    case 'create_transaction': return { target_doctype: config.target_doctype || '', child_table: config.child_table || '', line_source: config.line_source || 'cart' }
    case 'workflow_transition': return { doctype: config.doctype || config.target_doctype || '', transition: config.transition || '' }
    case 'local_cart_add': return { product_field: config.product_field || 'product', rate_field: config.rate_field || 'rate' }
    case 'local_cart_remove': return { item_key: config.item_key || 'name' }
    case 'local_state_set': return { key: config.key || '', value: config.value || '' }
    case 'call_script': return { script: config.script || '' }
    case 'call_webhook': return { webhook: config.webhook || '' }
    default: return config
  }
}

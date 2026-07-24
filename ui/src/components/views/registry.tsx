import React, { type ComponentType } from 'react'
import type { ViewComponent } from '@/lib/api/views'

export interface ViewComponentProps {
  config: ViewComponent
  data?: any
  isLoading?: boolean
  disabled?: boolean
  readonly?: boolean
  children?: React.ReactNode
  onAction: (actionId: string, context: Record<string, any>) => void
}

type ComponentEntry = {
  component: ComponentType<ViewComponentProps>
  preload?: () => Promise<any>
}

// Lazy imports for Phase 3-4 components.
const KanbanBoard = React.lazy(() => import('./KanbanBoard'))
const ApprovalQueue = React.lazy(() => import('./ApprovalQueue'))
const CalendarView = React.lazy(() => import('./CalendarView'))
const DashboardGrid = React.lazy(() => import('./DashboardGrid'))
const MetricCard = React.lazy(() => import('./MetricCard'))
const ChartView = React.lazy(() => import('./ChartView'))
const ScannerInput = React.lazy(() => import('./ScannerInput'))
const ProductGrid = React.lazy(() => import('./ProductGrid'))
const CartPanel = React.lazy(() => import('./CartPanel'))
const PaymentPanel = React.lazy(() => import('./PaymentPanel'))
const ScannerCount = React.lazy(() => import('./ScannerCount'))
const DocumentPreview = React.lazy(() => import('./DocumentPreview'))
const ConfirmationStep = React.lazy(() => import('./ConfirmationStep'))
const ReceiptPreview = React.lazy(() => import('./ReceiptPreview'))
const Drawer = React.lazy(() => import('./Drawer'))
const CategoryTabs = React.lazy(() => import('./CategoryTabs'))
const Tabs = React.lazy(() => import('./Tabs'))
const LineItemBuilder = React.lazy(() => import('./LineItemBuilder'))
const Wizard = React.lazy(() => import('./Wizard'))
const Checklist = React.lazy(() => import('./Checklist'))
const RecentRecords = React.lazy(() => import('./RecentRecords'))
const PublicForm = React.lazy(() => import('./PublicForm'))
const WorkspaceDashboard = React.lazy(() => import('./WorkspaceDashboard'))
const PrintLayout = React.lazy(() => import('./PrintLayout'))

export const COMPONENT_REGISTRY: Record<string, ComponentEntry> = {
  record_table:     { component: RecordTable },
  record_form:      { component: RecordForm },
  record_detail:    { component: RecordDetail },
  record_list:      { component: RecordList },
  record_cards:     { component: RecordCards },
  filter_bar:       { component: FilterBar },
  search_box:       { component: SearchBox },
  workflow_actions: { component: WorkflowActions },
  split_view:       { component: SplitView },
  kanban_board:     { component: KanbanBoard },
  approval_queue:   { component: ApprovalQueue },
  calendar_view:    { component: CalendarView },
  dashboard_grid:   { component: DashboardGrid },
  metric_card:      { component: MetricCard },
  chart:            { component: ChartView },
  scanner_input:    { component: ScannerInput },
  product_grid:     { component: ProductGrid },
  cart_panel:       { component: CartPanel },
  payment_panel:    { component: PaymentPanel },
  scanner_count:    { component: ScannerCount },
  document_preview: { component: DocumentPreview },
  confirmation_step:{ component: ConfirmationStep },
  receipt_preview:  { component: ReceiptPreview },
  drawer:           { component: Drawer },
  category_tabs:    { component: CategoryTabs },
  tabs:             { component: Tabs },
  line_item_builder:{ component: LineItemBuilder },
  wizard:           { component: Wizard },
  checklist:           { component: Checklist },
  recent_records:      { component: RecentRecords },
  public_form:         { component: PublicForm },
  workspace_dashboard: { component: WorkspaceDashboard },
  print_layout:        { component: PrintLayout },
}

// ====================================================================
// Core components
// ====================================================================

function RecordTable(props: ViewComponentProps) {
  const { data, isLoading } = props
  if (isLoading) return <div className="p-4 text-muted-foreground">Loading...</div>
  if (!data?.data?.length) return <div className="p-4 text-muted-foreground text-sm">No records</div>
  const columns = props.config.desktop_columns || Object.keys(data.data[0] || {}).filter(k => !k.startsWith('_'))
  return (
    <div className="overflow-x-auto rounded-lg border">
      <table className="w-full text-sm">
        <thead className="bg-muted/50">
          <tr>{columns.map((col: string) => <th key={col} className="px-4 py-2 text-left font-medium">{col}</th>)}</tr>
        </thead>
        <tbody>
          {data.data.map((row: any, i: number) => (
            <tr key={row.name || i} className="border-t hover:bg-muted/30 cursor-pointer"
              onClick={() => props.onAction('select', { name: row.name, row })}>
              {columns.map((col: string) => <td key={col} className="px-4 py-2">{row[col] != null ? String(row[col]) : '—'}</td>)}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function RecordForm(props: ViewComponentProps) {
  return <div className="rounded-lg border p-4 text-sm text-muted-foreground">Form: {props.config.source_doctype || props.config.label}</div>
}

function RecordDetail(props: ViewComponentProps) {
  const { data, config } = props
  return <div className="rounded-lg border p-4"><h3 className="font-semibold text-sm">{config.label || 'Detail'}</h3>
    {data ? <pre className="mt-2 text-xs max-h-96 overflow-auto">{JSON.stringify(data, null, 2)}</pre> : <p className="text-sm text-muted-foreground mt-2">Select a record</p>}
  </div>
}

function RecordList(props: ViewComponentProps) {
  const { data, isLoading } = props
  if (isLoading) return <div className="p-4 text-muted-foreground text-sm">Loading...</div>
  if (!data?.data?.length) return <div className="p-4 text-sm text-muted-foreground">No records</div>
  return <div className="space-y-1">{data.data.map((row: any, i: number) => (
    <div key={row.name || i} className="cursor-pointer rounded-md px-3 py-2 hover:bg-muted/50 text-sm"
      onClick={() => props.onAction('select', { name: row.name, row })}>{row.name}</div>
  ))}</div>
}

function RecordCards(props: ViewComponentProps) {
  const { data, isLoading } = props
  if (isLoading) return <div className="p-4 text-muted-foreground text-sm">Loading...</div>
  if (!data?.data?.length) return <div className="p-4 text-sm text-muted-foreground">No records</div>
  return <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
    {data.data.map((row: any, i: number) => (
      <div key={row.name || i} className="cursor-pointer rounded-lg border p-3 hover:bg-muted/30" onClick={() => props.onAction('select', { name: row.name, row })}>
        <p className="font-medium text-sm">{row.name}</p></div>))}
  </div>
}

function FilterBar(props: ViewComponentProps) {
  return <div className="flex flex-wrap gap-2 rounded-lg border bg-card p-3">
    <input
      type="text"
      placeholder="Search..."
      className="rounded-md border px-3 py-1.5 text-sm"
      onChange={(e) => props.onAction('search', { value: e.target.value })}
    />
  </div>
}

function SearchBox(props: ViewComponentProps) {
  return (
    <input
      type="text"
      placeholder="Search..."
      className="w-full rounded-md border px-3 py-2 text-sm"
      autoFocus
      onChange={(e) => props.onAction('search', { value: e.target.value })}
    />
  )
}

function WorkflowActions(props: ViewComponentProps) {
  return <div className="flex gap-2">
    <button className="rounded-md bg-primary px-4 py-1.5 text-sm text-primary-foreground" onClick={() => props.onAction('submit', {})}>Submit</button>
    <button className="rounded-md border px-4 py-1.5 text-sm">Cancel</button>
  </div>
}

function SplitView(props: ViewComponentProps) {
  return <div className="grid h-full gap-4 lg:grid-cols-[280px_1fr] xl:grid-cols-[300px_1fr_260px]">{props.children}</div>
}

function Placeholder(props: ViewComponentProps) {
  return <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">{props.config.type} (coming soon)</div>
}

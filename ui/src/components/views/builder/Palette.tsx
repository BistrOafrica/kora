import { useState } from 'react'
import { useBuilderStore } from '@/lib/builder-store'
import {
  CalendarDays,
  ChartLine,
  ChartNoAxesColumn,
  CheckSquare,
  CircleCheck,
  ClipboardList,
  Clock,
  Columns3,
  CreditCard,
  FileText,
  FolderTree,
  GitBranch,
  Grid3X3,
  LayoutDashboard,
  List,
  ListChecks,
  ListPlus,
  PanelLeft,
  PanelRight,
  PanelTop,
  PanelsTopLeft,
  Printer,
  ReceiptText,
  ScanBarcode,
  ScanLine,
  Search,
  ShieldCheck,
  ShoppingCart,
  SlidersHorizontal,
  SquarePen,
  Table,
  type LucideIcon,
} from 'lucide-react'

interface PaletteItem {
  type: string
  icon: LucideIcon
  label: string
  container?: boolean
}

interface PaletteCategory {
  id: string
  label: string
  items: PaletteItem[]
}

const CATEGORIES: PaletteCategory[] = [
  { id: 'collection', label: 'Collection', items: [
    { type: 'record_table', icon: Table, label: 'Table' },
    { type: 'record_list', icon: List, label: 'List' },
    { type: 'record_cards', icon: PanelsTopLeft, label: 'Cards' },
    { type: 'filter_bar', icon: SlidersHorizontal, label: 'Filter Bar' },
    { type: 'search_box', icon: Search, label: 'Search' },
  ] },
  { id: 'record', label: 'Record', items: [
    { type: 'record_form', icon: SquarePen, label: 'Form' },
    { type: 'record_detail', icon: FileText, label: 'Detail' },
    { type: 'workflow_actions', icon: GitBranch, label: 'Workflow Actions' },
    { type: 'recent_records', icon: Clock, label: 'Recent Records' },
  ] },
  { id: 'flow', label: 'Flow', items: [
    { type: 'kanban_board', icon: Columns3, label: 'Kanban' },
    { type: 'approval_queue', icon: CircleCheck, label: 'Approval Queue' },
  ] },
  { id: 'time', label: 'Time', items: [{ type: 'calendar_view', icon: CalendarDays, label: 'Calendar' }] },
  { id: 'summary', label: 'Summary', items: [
    { type: 'dashboard_grid', icon: LayoutDashboard, label: 'Dashboard', container: true },
    { type: 'metric_card', icon: ChartNoAxesColumn, label: 'Metric Card' },
    { type: 'chart', icon: ChartLine, label: 'Chart' },
    { type: 'workspace_dashboard', icon: LayoutDashboard, label: 'Workspace Dashboard', container: true },
  ] },
  { id: 'action', label: 'Action', items: [
    { type: 'scanner_input', icon: ScanBarcode, label: 'Scanner' },
    { type: 'product_grid', icon: Grid3X3, label: 'Product Grid' },
    { type: 'cart_panel', icon: ShoppingCart, label: 'Cart' },
    { type: 'payment_panel', icon: CreditCard, label: 'Payment' },
    { type: 'scanner_count', icon: ScanLine, label: 'Scanner Count' },
    { type: 'line_item_builder', icon: ListPlus, label: 'Line Items' },
    { type: 'confirmation_step', icon: ShieldCheck, label: 'Confirmation' },
  ] },
  { id: 'input', label: 'Input', items: [
    { type: 'public_form', icon: ClipboardList, label: 'Public Form' },
    { type: 'wizard', icon: ListChecks, label: 'Wizard', container: true },
    { type: 'checklist', icon: CheckSquare, label: 'Checklist' },
  ] },
  { id: 'output', label: 'Output', items: [
    { type: 'document_preview', icon: FileText, label: 'Document Preview' },
    { type: 'receipt_preview', icon: ReceiptText, label: 'Receipt' },
    { type: 'print_layout', icon: Printer, label: 'Print Layout', container: true },
  ] },
  { id: 'layout', label: 'Layout', items: [
    { type: 'split_view', icon: PanelLeft, label: 'Split View', container: true },
    { type: 'tabs', icon: PanelTop, label: 'Tabs', container: true },
    { type: 'drawer', icon: PanelRight, label: 'Drawer', container: true },
    { type: 'category_tabs', icon: FolderTree, label: 'Category Tabs' },
  ] },
]

export function Palette() {
  const [search, setSearch] = useState('')
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})
  const addComponent = useBuilderStore((s) => s.addComponent)

  const toggle = (id: string) => setCollapsed((c) => ({ ...c, [id]: !c[id] }))
  const handleDragStart = (e: React.DragEvent, type: string) => {
    e.dataTransfer.setData('component-type', type)
    e.dataTransfer.effectAllowed = 'copy'
  }

  const filtered = CATEGORIES.map((cat) => ({
    ...cat,
    items: cat.items.filter((item) => !search || item.label.toLowerCase().includes(search.toLowerCase()) || item.type.includes(search.toLowerCase())),
  })).filter((cat) => cat.items.length > 0)

  return (
    <div className="h-full flex flex-col border-r bg-muted/10">
      <div className="p-3 border-b">
        <div className="relative">
          <Search className="pointer-events-none absolute left-2 top-2 h-3.5 w-3.5 text-muted-foreground" />
          <input type="text" placeholder="Search components..." className="w-full rounded-md border px-2 py-1.5 pl-7 text-xs" value={search} onChange={(e) => setSearch(e.target.value)} />
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-2 space-y-1">
        {filtered.map((cat) => (
          <div key={cat.id}>
            <button className="flex items-center gap-1 w-full px-1 py-1 text-xs font-semibold text-muted-foreground uppercase tracking-wide hover:text-foreground" onClick={() => toggle(cat.id)}>
              {collapsed[cat.id] ? <PanelRight className="h-3 w-3" /> : <PanelTop className="h-3 w-3" />}
              {cat.label}
            </button>
            {!collapsed[cat.id] && (
              <div className="space-y-0.5 mt-0.5">
                {cat.items.map((item) => {
                  const Icon = item.icon
                  return (
                    <div key={item.type} draggable onDragStart={(e) => handleDragStart(e, item.type)} onClick={() => {
                      const regions = getRegionsForLayout(useBuilderStore.getState().working?.view.layout || 'single')
                      addComponent(regions[0], item.type, useBuilderStore.getState().working?.view.components.length || 0)
                    }} className={`flex items-center gap-2 px-2 py-1.5 rounded-md text-xs cursor-grab hover:bg-muted/50 transition-colors ${item.container ? 'border border-dashed border-muted-foreground/30' : ''}`}>
                      <Icon className="h-4 w-4 text-muted-foreground shrink-0" />
                      <span className="truncate">{item.label}</span>
                      {item.container && <span className="ml-auto text-[10px] text-muted-foreground">container</span>}
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

function getRegionsForLayout(layout: string): string[] {
  switch (layout) {
    case 'two_panel': return ['main', 'side']
    case 'three_panel': return ['left', 'main', 'right']
    case 'grid': return ['main']
    default: return ['main']
  }
}

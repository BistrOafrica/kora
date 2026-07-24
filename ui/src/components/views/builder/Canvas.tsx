import { useBuilderStore } from '@/lib/builder-store'
import {
  CalendarDays,
  ChartLine,
  ChartNoAxesColumn,
  CheckSquare,
  CircleCheck,
  ClipboardList,
  Clock,
  Columns2,
  Columns3,
  CreditCard,
  FileText,
  FolderTree,
  GitBranch,
  Grid3X3,
  GripVertical,
  LayoutDashboard,
  LayoutGrid,
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
  Trash2,
  Component,
  type LucideIcon,
} from 'lucide-react'
import type { ViewComponent } from '@/lib/api/views'

const COMPONENT_ICONS: Record<string, LucideIcon> = {
  record_table: Table,
  record_list: List,
  record_cards: PanelsTopLeft,
  record_form: SquarePen,
  record_detail: FileText,
  filter_bar: SlidersHorizontal,
  search_box: Search,
  workflow_actions: GitBranch,
  split_view: PanelLeft,
  kanban_board: Columns3,
  approval_queue: CircleCheck,
  calendar_view: CalendarDays,
  dashboard_grid: LayoutDashboard,
  metric_card: ChartNoAxesColumn,
  chart: ChartLine,
  scanner_input: ScanBarcode,
  product_grid: Grid3X3,
  cart_panel: ShoppingCart,
  payment_panel: CreditCard,
  scanner_count: ScanLine,
  document_preview: FileText,
  confirmation_step: ShieldCheck,
  receipt_preview: ReceiptText,
  drawer: PanelRight,
  category_tabs: FolderTree,
  tabs: PanelTop,
  line_item_builder: ListPlus,
  wizard: ListChecks,
  checklist: CheckSquare,
  recent_records: Clock,
  public_form: ClipboardList,
  workspace_dashboard: LayoutDashboard,
  print_layout: Printer,
}

const LAYOUT_ICONS: Record<string, LucideIcon> = {
  single: LayoutGrid,
  two_panel: Columns2,
  three_panel: Columns3,
  grid: LayoutDashboard,
}

const CONTAINER_TYPES = new Set(['dashboard_grid', 'split_view', 'tabs', 'wizard', 'drawer', 'print_layout', 'workspace_dashboard'])

function getRegions(layout: string): { id: string; label: string; width: string }[] {
  switch (layout) {
    case 'two_panel':
      return [{ id: 'main', label: 'Main Content', width: '1fr' }, { id: 'side', label: 'Sidebar', width: '320px' }]
    case 'three_panel':
      return [{ id: 'left', label: 'Left Panel', width: '240px' }, { id: 'main', label: 'Main Content', width: '1fr' }, { id: 'right', label: 'Right Panel', width: '280px' }]
    case 'grid':
      return [{ id: 'main', label: 'Content Grid', width: '1fr' }]
    default:
      return [{ id: 'main', label: 'Content', width: '1fr' }]
  }
}

export function Canvas() {
  const working = useBuilderStore((s) => s.working)
  const selectedId = useBuilderStore((s) => s.selectedComponentId)
  const selectComponent = useBuilderStore((s) => s.selectComponent)
  const removeComponent = useBuilderStore((s) => s.removeComponent)
  const addComponent = useBuilderStore((s) => s.addComponent)
  const addChildComponent = useBuilderStore((s) => s.addChildComponent)

  if (!working?.view) {
    return <div className="h-full flex items-center justify-center text-sm text-muted-foreground">Loading view...</div>
  }

  const layout = working.view.layout || 'single'
  const components = working.view.components || []
  const LayoutIcon = LAYOUT_ICONS[layout] || LayoutGrid
  const regions = getRegions(layout)

  const handleDrop = (e: React.DragEvent, region: string) => {
    e.preventDefault()
    const type = e.dataTransfer.getData('component-type')
    if (type) addComponent(region, type, components.filter((c) => c.region === region).length)
  }

  return (
    <div className="h-full flex flex-col">
      <div className="flex items-center gap-2 px-4 py-2 border-b bg-muted/5 shrink-0">
        <LayoutIcon className="h-4 w-4 text-muted-foreground" />
        <span className="text-xs font-medium text-muted-foreground uppercase">{layout.replace('_', ' ')} Layout</span>
        <span className="text-[10px] text-muted-foreground/60 ml-2">{regions.length} region{regions.length !== 1 ? 's' : ''} · {components.length} component{components.length !== 1 ? 's' : ''}</span>
      </div>
      <div className="flex-1 p-6 gap-4 overflow-auto" style={{ display: 'grid', gridTemplateColumns: regions.map((r) => r.width).join(' '), gridTemplateRows: '1fr' }}>
        {regions.map((region) => {
          const regionComps = components.filter((c) => c.region === region.id)
          return (
            <div key={region.id} className="flex flex-col min-h-[300px]" onDragOver={(e) => e.preventDefault()} onDrop={(e) => handleDrop(e, region.id)}>
              <div className="flex items-center gap-2 mb-2 shrink-0">
                <div className="h-2 w-2 rounded-full bg-primary/40" />
                <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{region.label}</span>
                {regionComps.length > 0 && <span className="text-[10px] text-muted-foreground bg-muted px-1.5 py-0.5 rounded-full">{regionComps.length}</span>}
              </div>
              <div className={`flex-1 rounded-xl border-2 transition-all duration-150 ${regionComps.length === 0 ? 'border-dashed border-muted-foreground/15 bg-muted/5 flex items-center justify-center' : 'border-transparent bg-muted/5'}`}>
                {regionComps.length === 0 ? (
                  <div className="text-center p-6">
                    <p className="text-xs text-muted-foreground/60 font-medium">Drop components here</p>
                    <p className="text-[10px] text-muted-foreground/40 mt-1">Drag from the palette or click a component to add it</p>
                  </div>
                ) : (
                  <div className="w-full p-3 space-y-2">
                    {regionComps.map((comp) => (
                      <ComponentBox
                        key={comp.id}
                        component={comp}
                        selectedId={selectedId}
                        onSelect={selectComponent}
                        onRemove={removeComponent}
                        onAddChild={addChildComponent}
                        depth={0}
                      />
                    ))}
                  </div>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function ComponentBox({ component, selectedId, onSelect, onRemove, onAddChild, depth }: {
  component: ViewComponent
  selectedId: string | null
  onSelect: (id: string) => void
  onRemove: (id: string) => void
  onAddChild: (parentId: string, type: string) => void
  depth: number
}) {
  const Icon = COMPONENT_ICONS[component.type] || Component
  const hasChildren = !!component.components?.length
  const isSelected = selectedId === component.id
  const isContainer = CONTAINER_TYPES.has(component.type)

  const handleChildDrop = (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    const type = e.dataTransfer.getData('component-type')
    if (type) onAddChild(component.id, type)
  }

  return (
    <div>
      <div
        className={`group flex items-center gap-2 px-3 py-2 rounded-lg border transition-all ${isSelected ? 'border-primary bg-primary/5 ring-2 ring-primary/20 shadow-sm' : 'border-border bg-card hover:border-muted-foreground/30 hover:shadow-sm'}`}
        style={{ marginLeft: depth * 18 }}
        onClick={(e) => { e.stopPropagation(); onSelect(component.id) }}
      >
        <GripVertical className="h-3.5 w-3.5 text-muted-foreground/40 shrink-0 cursor-grab opacity-0 group-hover:opacity-100 transition-opacity" />
        <Icon className="h-4 w-4 text-muted-foreground shrink-0" />
        <div className="flex-1 min-w-0">
          <p className="text-xs font-medium truncate">{component.label || component.type.replace(/_/g, ' ')}</p>
          <div className="flex items-center gap-2 mt-0.5">
            <span className="text-[10px] text-muted-foreground font-mono">{component.type}</span>
            {component.source_doctype && <span className="text-[10px] text-muted-foreground">· {component.source_doctype}</span>}
            {hasChildren && <span className="text-[10px] text-muted-foreground">· {component.components!.length} nested</span>}
          </div>
        </div>
        <button className="p-1 rounded-md hover:bg-destructive/10 text-muted-foreground/40 hover:text-destructive shrink-0 opacity-0 group-hover:opacity-100 transition-all" onClick={(e) => { e.stopPropagation(); onRemove(component.id) }} title="Remove component">
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </div>

      {isContainer && (
        <div className="mt-1 space-y-1 rounded-md border border-dashed border-muted-foreground/20 bg-muted/10 p-2" style={{ marginLeft: (depth + 1) * 18 }} onDragOver={(e) => e.preventDefault()} onDrop={handleChildDrop}>
          {!hasChildren && <p className="px-2 py-1 text-[10px] text-muted-foreground">Drop child components here</p>}
          {component.components?.map((child) => (
            <ComponentBox key={child.id} component={child} selectedId={selectedId} onSelect={onSelect} onRemove={onRemove} onAddChild={onAddChild} depth={depth + 1} />
          ))}
        </div>
      )}
    </div>
  )
}

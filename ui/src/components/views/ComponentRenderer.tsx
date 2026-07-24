import { useMemo } from 'react'
import type { ViewComponent } from '@/lib/api/views'
import { COMPONENT_REGISTRY } from './registry'
import { useComponentData } from '@/lib/view-runtime'
import { useComponentRules } from '@/lib/view-rules'
import { handleAction } from '@/lib/view-actions'
import { Loader2 } from 'lucide-react'

interface ComponentRendererProps {
  component: ViewComponent
  isPublic: boolean
  viewName: string
  viewFilters?: Record<string, any>
  setViewFilter?: (key: string, value: any) => void
}

/**
 * ComponentRenderer resolves a ViewComponent config into a rendered
 * React component. It:
 * 1. Looks up the component type in the registry
 * 2. Fetches data based on bindings/filters
 * 3. Evaluates visibility/disabled rules
 * 4. Renders the component with resolved props
 * 5. Recursively renders nested children
 */
export function ComponentRenderer({
  component,
  isPublic,
  viewName,
  viewFilters = {},
  setViewFilter,
}: ComponentRendererProps) {
  const entry = COMPONENT_REGISTRY[component.type]

  // Data fetching
  const { data, isLoading } = useComponentData(component, isPublic)

  // Rule evaluation
  const rules = useComponentRules(component.rules, data)
  const filteredData = useMemo(() => applyViewFilters(data, viewFilters), [data, viewFilters])

  // Render children for container components
  const children = useMemo(() => {
    if (!component.components?.length) return null
    return component.components.map((child) => (
      <ComponentRenderer
        key={child.id}
        component={child}
        isPublic={isPublic}
        viewName={viewName}
        viewFilters={viewFilters}
        setViewFilter={setViewFilter}
      />
    ))
  }, [component.components, isPublic, viewName])

  // Preload heavy components on hover
  const handleMouseEnter = () => {
    if (entry?.preload) {
      entry.preload()
    }
  }

  if (rules.hidden) return null

  if (!entry) {
    return (
      <div className="rounded-lg border border-destructive/30 p-4 text-sm text-muted-foreground">
        Unknown component type: {component.type}
      </div>
    )
  }

  const Comp = entry.component

  const handleComponentAction = async (actionId: string, context: Record<string, any>) => {
    if (actionId === 'filter') {
      Object.entries(context).forEach(([key, value]) => setViewFilter?.(key, value))
      return
    }
    if (actionId === 'search') {
      setViewFilter?.('search', context.value || '')
      return
    }

    const action = component.actions?.find((a) => a.id === actionId)
    if (!action) return

    return handleAction(
      actionId,
      viewName,
      component.id,
      action.type,
      action.config,
      {
        viewName,
        doctype: component.source_doctype,
        name: context.name,
        data: context,
      },
    )
  }

  return (
    <div
      className="view-component"
      data-component-id={component.id}
      data-component-type={component.type}
      onMouseEnter={handleMouseEnter}
    >
      {isLoading && !data ? (
        <div className="flex items-center justify-center p-8">
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <Comp
          config={component}
          data={filteredData}
          isLoading={isLoading}
          disabled={rules.disabled}
          readonly={rules.readonly}
          onAction={handleComponentAction}
        >
          {children}
        </Comp>
      )}
    </div>
  )
}


function applyViewFilters(data: any, filters: Record<string, any>) {
  if (!data?.data || !Array.isArray(data.data)) return data

  const search = String(filters.search || '').trim().toLowerCase()
  const category = String(filters.category || '').trim()

  let rows = data.data
  if (category) {
    rows = rows.filter((row: any) => String(row.category || '') === category)
  }
  if (search) {
    rows = rows.filter((row: any) =>
      Object.values(row).some((value) => String(value ?? '').toLowerCase().includes(search)),
    )
  }

  return {
    ...data,
    data: rows,
    meta: data.meta ? { ...data.meta, total: rows.length } : data.meta,
  }
}

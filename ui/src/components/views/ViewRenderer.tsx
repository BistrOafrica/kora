import { useState } from 'react'
import { useParams, useSearch } from '@tanstack/react-router'
import { useViewConfig } from '@/lib/view-runtime'
import { ComponentRenderer } from './ComponentRenderer'
import { Skeleton } from '@/components/ui/skeleton'

/**
 * ViewRenderer reads a view config by route name and renders
 * its component tree. This is the entry point for all dynamic
 * view routes at /workspace/pages/$viewName.
 */
export function ViewRenderer() {
  const { viewName } = useParams({ from: '/workspace/pages/$viewName' })
  const search = useSearch({ strict: false }) as { version?: string }
  const [viewFilters, setViewFilters] = useState<Record<string, any>>({})
  const route = viewName.startsWith('/') ? viewName : `/${viewName}`
    const { data: viewConfig, isLoading, isError } = useViewConfig(route, search.version)

  if (isLoading) {
    return (
      <div className="p-8 space-y-4">
        <Skeleton className="h-8 w-48" />
        <div className="grid gap-4 lg:grid-cols-[1fr_300px]">
          <Skeleton className="h-96" />
          <Skeleton className="h-64" />
        </div>
      </div>
    )
  }

  if (isError || !viewConfig) {
    return (
      <div className="flex h-64 items-center justify-center">
        <p className="text-muted-foreground">
          View "{viewName}" not found.
        </p>
      </div>
    )
  }

  const view = viewConfig.view
  const components = viewConfig.components || view.components
  const isPublic = viewConfig.is_public || false

  return (
    <div className="p-4 md:p-6">
      {/* View header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">{view.label || view.name}</h1>
        {view.module && (
          <p className="text-sm text-muted-foreground">{view.module}</p>
        )}
      </div>

      {/* Layout */}
      <div className={layoutClass(view.layout)}>
        {components.map((comp) => (
          <ComponentRenderer
            key={comp.id}
            component={comp}
            isPublic={isPublic}
            viewName={view.name}
            viewFilters={viewFilters}
            setViewFilter={(key, value) => setViewFilters((prev) => ({ ...prev, [key]: value }))}
          />
        ))}
      </div>
    </div>
  )
}

function layoutClass(layout: string): string {
  switch (layout) {
    case 'two_panel':
      return 'grid gap-6 lg:grid-cols-[1fr_360px]'
    case 'three_panel':
      return 'grid gap-4 lg:grid-cols-[260px_1fr] xl:grid-cols-[280px_1fr_280px]'
    case 'grid':
      return 'grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3'
    case 'single':
    default:
      return 'space-y-4'
  }
}

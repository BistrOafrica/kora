import type { ViewComponentProps } from './registry'

/**
 * WorkspaceDashboard is a dashboard variant for workspace pages.
 * Shows key metrics relevant to the workspace's source doctype.
 * Delegates to child metric cards and recent records.
 */
export default function WorkspaceDashboard(props: ViewComponentProps) {
  const { config, children } = props

  return (
    <div className="space-y-6">
      {config.label && (
        <div>
          <h2 className="text-xl font-bold">{config.label}</h2>
          {config.bindings?.subtitle && (
            <p className="text-sm text-muted-foreground mt-1">{config.bindings.subtitle}</p>
          )}
        </div>
      )}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {children}
      </div>
    </div>
  )
}

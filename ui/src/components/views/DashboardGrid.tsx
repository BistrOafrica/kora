import type { ViewComponentProps } from './registry'

/**
 * DashboardGrid is a responsive container that renders child metric cards
 * and charts in a grid layout. It delegates rendering to its children.
 */
export default function DashboardGrid(props: ViewComponentProps) {
  return (
    <div className="space-y-4">
      {props.config.label && (
        <h3 className="text-sm font-semibold text-muted-foreground">{props.config.label}</h3>
      )}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {props.children}
      </div>
    </div>
  )
}

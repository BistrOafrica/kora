import type { ViewComponentProps } from './registry'

/**
 * CategoryTabs renders horizontal category filter tabs.
 * Tapping a category filters the sibling product_grid component.
 */
export default function CategoryTabs(props: ViewComponentProps) {
  const { data, config, onAction } = props

  const categories = data?.data || []
  const groupField = config.bindings?.group_field || 'category'
  const uniqueCategories = [...new Set(categories.map((r: any) => r[groupField]).filter(Boolean))] as string[]

  if (uniqueCategories.length === 0) {
    return <div className="flex gap-2 overflow-x-auto pb-2">
      {['All'].map(cat => (
        <button key={cat} className="px-3 py-1.5 text-sm rounded-full bg-muted hover:bg-muted/80 whitespace-nowrap"
          onClick={() => onAction('filter', { category: cat === 'All' ? '' : cat })}>{cat}</button>
      ))}
    </div>
  }

  return (
    <div className="flex gap-2 overflow-x-auto pb-2">
      <button className="px-3 py-1.5 text-sm rounded-full bg-primary text-primary-foreground whitespace-nowrap"
        onClick={() => onAction('filter', { category: '' })}>All</button>
      {uniqueCategories.map(cat => (
        <button key={cat} className="px-3 py-1.5 text-sm rounded-full bg-muted hover:bg-muted/80 whitespace-nowrap"
          onClick={() => onAction('filter', { category: cat })}>{cat}</button>
      ))}
    </div>
  )
}

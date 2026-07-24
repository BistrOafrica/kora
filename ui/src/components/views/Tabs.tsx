import { useState } from 'react'
import type { ViewComponentProps } from './registry'
import { X } from 'lucide-react'

/**
 * Tabs renders children as tab panels with clickable tab headers.
 * Each child component's label is used as the tab title.
 */
export default function Tabs(props: ViewComponentProps) {
  const { config, children } = props
  const childArray = React.Children.toArray(children) as React.ReactElement[]
  const [active, setActive] = useState(0)

  const tabs = config.components || []
  if (tabs.length === 0 && childArray.length === 0) {
    return <div className="text-sm text-muted-foreground p-4">No tabs configured</div>
  }

  const labels = tabs.map(c => c.label || c.id || `Tab ${tabs.indexOf(c) + 1}`)

  return (
    <div className="rounded-lg border">
      <div className="flex border-b bg-muted/20 overflow-x-auto">
        {labels.map((label, i) => (
          <button
            key={i}
            className={`px-4 py-2 text-sm font-medium whitespace-nowrap transition-colors ${
              i === active ? 'border-b-2 border-primary text-primary bg-background' : 'text-muted-foreground hover:text-foreground'
            }`}
            onClick={() => setActive(i)}
          >
            {label}
          </button>
        ))}
      </div>
      <div className="p-4">
        {childArray[active] || <p className="text-sm text-muted-foreground">Empty tab</p>}
      </div>
    </div>
  )
}

import React from 'react'

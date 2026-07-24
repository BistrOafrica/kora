import { useState } from 'react'
import type { ViewComponentProps } from './registry'
import { Button } from '@/components/ui/button'
import { ChevronLeft, ChevronRight, Check } from 'lucide-react'

/**
 * Wizard renders a multi-step guided flow. Each child component
 * is one step. Navigation between steps is handled internally.
 */
export default function Wizard(props: ViewComponentProps) {
  const { config, children, onAction } = props
  const childArray = React.Children.toArray(children) as React.ReactElement[]
  const [step, setStep] = useState(0)
  const steps = config.components || []
  const total = Math.max(childArray.length, steps.length, 1)
  const stepLabels = steps.map(s => s.label || `Step ${steps.indexOf(s) + 1}`)

  return (
    <div className="rounded-lg border">
      <div className="flex items-center px-4 py-3 border-b bg-muted/20 gap-1">
        {Array.from({ length: total }).map((_, i) => (
          <div key={i} className="flex items-center gap-1">
            <div className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-medium ${
              i <= step ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'
            }`}>{i + 1}</div>
            <span className={`text-xs ${i <= step ? 'font-medium' : 'text-muted-foreground'}`}>{stepLabels[i] || `Step ${i + 1}`}</span>
            {i < total - 1 && <div className={`w-4 h-px ${i < step ? 'bg-primary' : 'bg-muted-foreground/30'}`} />}
          </div>
        ))}
      </div>
      <div className="p-6 min-h-[200px]">
        {childArray[step] || <p className="text-muted-foreground text-sm">Step {step + 1} content</p>}
      </div>
      <div className="flex justify-between px-4 py-3 border-t bg-muted/10">
        <Button variant="outline" size="sm" disabled={step === 0} onClick={() => setStep(step - 1)}>
          <ChevronLeft className="h-4 w-4 mr-1" />Back
        </Button>
        {step < total - 1 ? (
          <Button size="sm" onClick={() => setStep(step + 1)}>Next<ChevronRight className="h-4 w-4 ml-1" /></Button>
        ) : (
          <Button size="sm" onClick={() => onAction('submit', {})}><Check className="h-4 w-4 mr-1" />Finish</Button>
        )}
      </div>
    </div>
  )
}

import React from 'react'

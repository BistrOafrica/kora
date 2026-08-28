import type { ReactNode } from 'react'
import { CircleHelp } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

export function HelpTooltip({
  label,
  children,
  className,
}: {
  label: string
  children: ReactNode
  className?: string
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        aria-label={label}
        className={cn('inline-flex align-middle text-muted-foreground hover:text-foreground', className)}
        render={(
          <span
            role="button"
            tabIndex={0}
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => event.stopPropagation()}
          />
        )}
      >
        <CircleHelp className="h-3.5 w-3.5" />
      </TooltipTrigger>
      <TooltipContent side="top" align="start">
        {children}
      </TooltipContent>
    </Tooltip>
  )
}

export function FieldLabelWithHelp({
  label,
  help,
  className,
}: {
  label: string
  help: ReactNode
  className?: string
}) {
  return (
    <Label className={cn('inline-flex items-center gap-1.5', className)}>
      {label}
      <HelpTooltip label={`${label} help`}>{help}</HelpTooltip>
    </Label>
  )
}

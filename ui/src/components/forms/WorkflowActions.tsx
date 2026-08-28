import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { fetchDoctypeSchema } from '@/lib/api/system'
import { submitWorkflowAction } from '@/lib/api/resources'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Loader2, GitBranch, AlertCircle } from 'lucide-react'
import type { WorkflowTransition } from '@/types/kora'

interface Props {
  doctype: string
  name: string
  currentState: string
}

const stateColors: Record<string, string> = {
  'Draft':        'bg-gray-100 text-gray-700',
  'Lead':         'bg-slate-100 text-slate-700',
  'Qualified':    'bg-sky-100 text-sky-700',
  'Proposal':     'bg-amber-100 text-amber-700',
  'Negotiation':  'bg-orange-100 text-orange-700',
  'Closed Won':   'bg-emerald-100 text-emerald-700',
  'Closed Lost':  'bg-red-100 text-red-700',
}

export function WorkflowActions({ doctype, name, currentState }: Props) {
  const queryClient = useQueryClient()
  const [acting, setActing] = useState<string | null>(null)
  const [lastError, setLastError] = useState<string | null>(null)

  // Load the full workflow to get ALL transitions, then filter by current state.
  // Don't use the state-filtered endpoint — it evaluates conditions against an empty doc.
  const schemaQuery = useQuery({
    queryKey: ['doctype', doctype],
    queryFn: () => fetchDoctypeSchema(doctype),
    staleTime: 5 * 60_000,
  })

  const allTransitions: WorkflowTransition[] = schemaQuery.data?.workflow?.transitions ?? []

  // Show transitions whose "from" matches the current state.
  const available = allTransitions.filter((t) => t.from === currentState)

  const handleAction = async (action: string) => {
    setActing(action)
    setLastError(null)
    try {
      await submitWorkflowAction(doctype, name, action)
      queryClient.invalidateQueries({ queryKey: ['resource', doctype, name] })
      queryClient.invalidateQueries({ queryKey: ['resource', doctype] })
      queryClient.invalidateQueries({ queryKey: ['doctype', doctype] })
      queryClient.invalidateQueries({ queryKey: ['manifest-resource'] })
    } catch (err: any) {
      setLastError(err.message || 'Action failed')
    } finally {
      setActing(null)
    }
  }

  if (!currentState || currentState === 'null') return null
  if (!schemaQuery.data?.workflow) return null

  return (
    <div className="mb-6 rounded-lg border bg-muted/30 p-4">
      <div className="flex items-center gap-3 mb-3">
        <GitBranch className="h-4 w-4 text-muted-foreground" />
        <span className="text-sm font-medium">Pipeline</span>
        <Badge className={stateColors[currentState] || 'bg-gray-100 text-gray-700'}>
          {currentState}
        </Badge>
      </div>

      {available.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {available.map((t) => (
            <Button
              key={t.action}
              variant="outline"
              size="sm"
              onClick={() => handleAction(t.action)}
              disabled={acting !== null}
              title={t.condition ? `Condition: ${t.condition}` : undefined}
            >
              {acting === t.action ? (
                <Loader2 className="mr-1.5 h-3 w-3 animate-spin" />
              ) : null}
              {t.action}
            </Button>
          ))}
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">
          No actions available from this state.
        </p>
      )}

      {lastError && (
        <div className="mt-3 flex items-start gap-2 rounded bg-destructive/10 px-3 py-2 text-xs text-destructive">
          <AlertCircle className="mt-0.5 h-3 w-3 shrink-0" />
          {lastError}
        </div>
      )}
    </div>
  )
}

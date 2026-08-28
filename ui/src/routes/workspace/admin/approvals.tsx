import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { toast } from '@/components/ui/Toast'
import { fetchAiApprovals, grantAiApproval, type AiApproval } from '@/lib/api/approvals'
import { CheckCircle2, Clock3, ShieldAlert } from 'lucide-react'
import { HelpTooltip } from '@/components/ui/help-tooltip'

export default function AdminApprovalsPage() {
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<AiApproval | null>(null)
  const [granting, setGranting] = useState(false)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['ai', 'approvals', 'pending'],
    queryFn: () => fetchAiApprovals('pending_approval'),
    staleTime: 15_000,
  })

  const handleGrant = async () => {
    if (!selected) return
    setGranting(true)
    try {
      await grantAiApproval(selected.id)
      queryClient.invalidateQueries({ queryKey: ['ai', 'approvals', 'pending'] })
      toast('success', `Granted ${selected.tool_name}`)
      setSelected(null)
    } catch (err: any) {
      toast('error', err.message || 'Failed to grant approval')
    } finally {
      setGranting(false)
    }
  }

  return (
    <div className="p-4 md:p-8 space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
            Approvals
            <HelpTooltip label="Approvals help">Review blocked AI actions and grant them only after human confirmation.</HelpTooltip>
          </h1>
        </div>
        <Badge variant="outline" className="gap-1"><ShieldAlert className="h-3.5 w-3.5" /> Pending approvals</Badge>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          <Skeleton className="h-28 w-full" />
          <Skeleton className="h-28 w-full" />
        </div>
      ) : isError ? (
        <Card className="border-dashed">
          <CardContent className="p-6">
            <p className="font-medium">We couldn’t load approvals.</p>
            <p className="mt-1 text-sm text-muted-foreground">{error instanceof Error ? error.message : 'Try again.'}</p>
            <Button variant="outline" size="sm" className="mt-4" onClick={() => refetch()}>Retry</Button>
          </CardContent>
        </Card>
      ) : !data?.length ? (
        <Card className="border-dashed">
          <CardContent className="flex items-center gap-3 p-6 text-sm text-muted-foreground">
            <CheckCircle2 className="h-5 w-5" />
            No pending approvals.
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {data.map((approval) => (
            <Card key={approval.id}>
              <CardHeader className="pb-3">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0">
                    <CardTitle className="text-base">{approval.tool_name}</CardTitle>
                    <CardDescription className="mt-1 break-all font-mono text-xs">
                      operation {approval.operation_id} · actor {approval.actor_principal_id}
                    </CardDescription>
                  </div>
                  <Badge variant="secondary">{approval.state}</Badge>
                </div>
              </CardHeader>
              <CardContent className="space-y-3 pt-0">
                <div className="grid gap-2 text-sm md:grid-cols-2">
                  <p className="text-muted-foreground"><span className="text-foreground">Type:</span> {approval.actor_principal_type}</p>
                  <p className="text-muted-foreground"><span className="text-foreground">Record version:</span> {approval.record_version}</p>
                  <p className="text-muted-foreground"><span className="text-foreground">Requested:</span> {approval.requested_at}</p>
                  <p className="text-muted-foreground"><span className="text-foreground">Expires:</span> {approval.expires_at || '—'}</p>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button size="sm" onClick={() => setSelected(approval)}>
                    <Clock3 className="mr-2 h-4 w-4" />
                    Grant approval
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <ConfirmDialog
        open={!!selected}
        onOpenChange={(open) => { if (!open) setSelected(null) }}
        title="Grant approval"
        description={selected ? <>Grant <strong>{selected.tool_name}</strong> for operation <code>{selected.operation_id}</code>?</> : undefined}
        confirmLabel="Grant"
        onConfirm={handleGrant}
        loading={granting}
      />
    </div>
  )
}

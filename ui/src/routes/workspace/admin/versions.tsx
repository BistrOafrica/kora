import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, KoraApiError } from '@/lib/api/client'
import {
  activateVersion,
  discardVersion,
  fetchConfigVersionPreview,
  fetchRollbackVersionPreview,
  rollbackVersion,
} from '@/lib/api/system'
import type {
  ConfigVersion,
  ConfigVersionPreview,
  RollbackVersionPreview,
} from '@/lib/api/system'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { History, Eye, Play, X, RotateCcw, AlertTriangle, RefreshCw } from 'lucide-react'
import { toast } from '@/components/ui/Toast'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'

type ConfirmAction =
  | { type: 'activate'; id: string }
  | { type: 'discard'; id: string }
  | { type: 'rollback'; id: string }
  | { type: 'activateAll' }
  | null

type LoadedPreview =
  | { actionType: 'activate' | 'activateAll'; data: ConfigVersionPreview | null }
  | { actionType: 'rollback'; data: RollbackVersionPreview | null }
  | { actionType: 'discard'; data: null }
  | null

export default function AdminVersionsPage() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['admin', 'versions'],
    queryFn: async () => {
      const result = await api.get<any[]>('/api/system/config/versions')
      return result as ConfigVersion[]
    },
    staleTime: 15_000,
  })

  const [acting, setActing] = useState<string | null>(null)
  const [viewingDiff, setViewingDiff] = useState<string | null>(null)
  const [diffData, setDiffData] = useState<any>(null)
  const [confirmAction, setConfirmAction] = useState<ConfirmAction>(null)
  const [preview, setPreview] = useState<LoadedPreview>(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [dialogError, setDialogError] = useState<string | null>(null)

  const drafts = useMemo(() => (data || []).filter(v => v.status === 'Draft'), [data])

  const selectedTarget = useMemo(() => {
    if (!confirmAction || !data) return null
    if (confirmAction.type === 'activateAll') {
      const sortedDrafts = drafts.slice().sort((a, b) => b.version - a.version)
      return sortedDrafts[0] || null
    }
    return data.find(v => v.id === confirmAction.id) || null
  }, [confirmAction, data, drafts])

  useEffect(() => {
    let cancelled = false

    async function loadPreview() {
      setDialogError(null)
      setPreview(null)

      if (!confirmAction) {
        setPreviewLoading(false)
        return
      }

      if (confirmAction.type === 'discard') {
        setPreviewLoading(false)
        setPreview({ actionType: 'discard', data: null })
        return
      }

      const target = selectedTarget
      if (!target) {
        setPreviewLoading(false)
        setDialogError('No target version is available for this action.')
        return
      }

      setPreviewLoading(true)
      try {
        if (cancelled) return

        if (confirmAction.type === 'rollback') {
          const result = await fetchRollbackVersionPreview(target.id)
          if (cancelled) return
          setPreview({ actionType: 'rollback', data: result })
        } else {
          const result = await fetchConfigVersionPreview(target.id)
          if (cancelled) return
          setPreview({
            actionType: confirmAction.type === 'activateAll' ? 'activateAll' : 'activate',
            data: result,
          })
        }
      } catch (e) {
        if (cancelled) return
        const message = e instanceof Error ? e.message : 'Failed to load impact preview'
        setDialogError(message)
      } finally {
        if (!cancelled) setPreviewLoading(false)
      }
    }

    void loadPreview()
    return () => {
      cancelled = true
    }
  }, [confirmAction, selectedTarget])

  const handleActivate = (id: string) => {
    setConfirmAction({ type: 'activate', id })
  }

  const handleDiscard = (id: string) => {
    setConfirmAction({ type: 'discard', id })
  }

  const handleRollback = (id: string) => {
    setConfirmAction({ type: 'rollback', id })
  }

  const viewDiff = async (id: string) => {
    if (viewingDiff === id) {
      setViewingDiff(null)
      setDiffData(null)
      return
    }
    setViewingDiff(id)
    try {
      const versions = data || []
      const currentIdx = versions.findIndex((v) => v.id === id)
      const prev = versions[currentIdx + 1]
      if (prev) {
        const result = await api.get<any>(`/api/system/config/diff?from=${prev.id}&to=${id}`)
        setDiffData(result)
      } else {
        setDiffData({ changes: [{ message: 'No previous version to compare against.', type: 'info' }] })
      }
    } catch (e) {
      setDiffData({ changes: [{ message: (e as Error).message, type: 'error' }] })
    }
  }

  const statusBadge = (status: string) => {
    switch (status) {
      case 'Active': return <Badge variant="default" className="bg-green-600">Active</Badge>
      case 'Draft': return <Badge variant="secondary" className="bg-amber-100 text-amber-800">Draft</Badge>
      case 'Superseded': return <Badge variant="outline">Superseded</Badge>
      default: return <Badge variant="outline">{status}</Badge>
    }
  }

  const getConfirmTitle = () => {
    switch (confirmAction?.type) {
      case 'activate': return 'Activate Version'
      case 'discard': return 'Discard Draft'
      case 'rollback': return 'Rollback Version'
      case 'activateAll': return 'Activate All Drafts'
      default: return ''
    }
  }

  const getConfirmDescription = () => {
    if (dialogError) return dialogError

    switch (confirmAction?.type) {
      case 'activate':
        return 'This will promote the selected draft to the live configuration.'
      case 'discard':
        return 'This draft will be marked as Superseded and removed from the pending queue.'
      case 'rollback':
        return 'This will replace the current config with the selected historical version.'
      case 'activateAll':
        return 'This will activate the latest draft and fold in all earlier draft changes.'
      default:
        return ''
    }
  }

  const getConfirmLabel = () => {
    switch (confirmAction?.type) {
      case 'discard': return 'Discard'
      case 'rollback': return 'Rollback'
      default: return 'Activate'
    }
  }

  const confirmVariant = confirmAction?.type === 'discard' ? 'destructive' : 'default'

  const handleConfirm = async () => {
    if (!confirmAction || acting) return

    const target = selectedTarget
    if (!target && confirmAction.type !== 'discard') {
      toast('error', 'No target version is available for this action.')
      return
    }

    setDialogError(null)
    setActing(target?.id || confirmAction.type)

    try {
      if (confirmAction.type === 'activate' || confirmAction.type === 'activateAll') {
        await activateVersion(target!.id)
      } else if (confirmAction.type === 'discard') {
        await discardVersion(confirmAction.id)
      } else if (confirmAction.type === 'rollback') {
        await rollbackVersion(confirmAction.id)
      }

      await refetch()
      setConfirmAction(null)
      setPreview(null)
    } catch (e) {
      if (e instanceof KoraApiError && e.status === 409) {
        setDialogError(
          `${e.message} Refresh the versions list, then reopen the draft from the latest active version before retrying.`,
        )
        await refetch()
      } else {
        const message = e instanceof Error ? e.message : 'Action failed'
        setDialogError(message)
      }
      toast('error', e instanceof Error ? e.message : 'Action failed')
    } finally {
      setActing(null)
    }
  }

  return (
    <div className="p-8 max-w-5xl">
      <div className="flex items-center gap-3 mb-6">
        <History className="h-6 w-6" />
        <h1 className="text-3xl font-bold tracking-tight">Config Versions</h1>
      </div>

      {isLoading && (
        <div className="space-y-2">
          {[1, 2, 3, 4].map((i) => <Skeleton key={i} className="h-16 w-full" />)}
        </div>
      )}

      {error && (
        <div className="border border-destructive/50 rounded-lg p-6 text-center">
          <p className="text-destructive font-medium">Failed to load versions</p>
          <Button variant="outline" className="mt-2" onClick={() => refetch()}>Retry</Button>
        </div>
      )}

      {data && data.length === 0 && (
        <div className="border-2 border-dashed rounded-lg p-12 text-center">
          <History className="h-12 w-12 mx-auto text-muted-foreground/40" />
          <h3 className="text-lg font-semibold mt-4">No versions yet</h3>
          <p className="text-muted-foreground mt-1">Config versions are created when doctypes are saved.</p>
        </div>
      )}

      {data && data.length > 0 && (
        <GroupedVersionList
          data={data}
          statusBadge={statusBadge}
          viewDiff={viewDiff}
          viewingDiff={viewingDiff}
          diffData={diffData}
          acting={acting}
          handleActivate={handleActivate}
          handleDiscard={handleDiscard}
          handleRollback={handleRollback}
          setConfirmAction={setConfirmAction}
        />
      )}

      <ConfirmDialog
        open={confirmAction !== null}
        onOpenChange={(open) => {
          if (!open && acting === null) {
            setConfirmAction(null)
            setPreview(null)
            setDialogError(null)
          }
        }}
        title={getConfirmTitle()}
        description={getConfirmDescription()}
        confirmLabel={getConfirmLabel()}
        variant={confirmVariant}
        loading={acting !== null || previewLoading}
        confirmDisabled={
          !!confirmAction &&
          confirmAction.type !== 'discard' &&
          !previewLoading &&
          !preview?.data
        }
        onConfirm={handleConfirm}
      >
        {confirmAction && (
          <div className="space-y-3">
            {previewLoading && (
              <div className="rounded-lg border bg-muted/40 px-4 py-3 text-sm text-muted-foreground flex items-center gap-2">
                <RefreshCw className="h-4 w-4 animate-spin" />
                Loading impact preview
              </div>
            )}

            {preview?.actionType === 'activate' || preview?.actionType === 'activateAll' ? (
              preview.data ? (
                <div className="rounded-lg border bg-muted/20 p-4 space-y-3">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <AlertTriangle className={`h-4 w-4 ${preview.data.is_breaking ? 'text-destructive' : 'text-amber-600'}`} />
                    <span>{preview.data.is_breaking ? 'Breaking changes detected' : 'Impact preview'}</span>
                  </div>
                  <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
                    <div className="text-muted-foreground">Doctypes</div>
                    <div>{preview.data.doctypes_in_snapshot}</div>
                    <div className="text-muted-foreground">Roles</div>
                    <div>{preview.data.roles_in_snapshot}</div>
                    <div className="text-muted-foreground">Permissions</div>
                    <div>{preview.data.permissions_in_snapshot}</div>
                    <div className="text-muted-foreground">Workflows</div>
                    <div>{preview.data.workflows_in_snapshot}</div>
                    <div className="text-muted-foreground">Newer active versions</div>
                    <div>{preview.data.newer_active_versions}</div>
                  </dl>
                  <div className="text-sm text-muted-foreground whitespace-pre-wrap">
                    {preview.data.diff_summary}
                  </div>
                  {preview.data.warning && (
                    <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-900">
                      {preview.data.warning}
                    </div>
                  )}
                </div>
              ) : null
            ) : preview?.actionType === 'rollback' ? (
              preview.data ? (
                <div className="rounded-lg border bg-muted/20 p-4 space-y-3">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <AlertTriangle className={`h-4 w-4 ${preview.data.is_breaking ? 'text-destructive' : 'text-amber-600'}`} />
                    <span>{preview.data.is_breaking ? 'Breaking changes detected' : 'Rollback impact preview'}</span>
                  </div>
                  <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
                    <div className="text-muted-foreground">Doctypes</div>
                    <div>{preview.data.doctypes_in_snapshot}</div>
                    <div className="text-muted-foreground">Changes</div>
                    <div>{preview.data.changes}</div>
                  </dl>
                  <div className="text-sm text-muted-foreground whitespace-pre-wrap">
                    {preview.data.diff_summary}
                  </div>
                  {preview.data.warning && (
                    <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-900">
                      {preview.data.warning}
                    </div>
                  )}
                  {preview.data.would_remove_doctypes.length > 0 && (
                    <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm">
                      <div className="font-medium text-destructive">Would remove doctypes</div>
                      <div className="mt-1 flex flex-wrap gap-2">
                        {preview.data.would_remove_doctypes.map((name) => (
                          <Badge key={name} variant="outline" className="border-destructive/30 text-destructive">
                            {name}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              ) : null
            ) : null}

            {confirmAction.type === 'discard' && (
              <div className="rounded-lg border bg-muted/20 px-4 py-3 text-sm text-muted-foreground">
                Discarding does not change the live schema. It only removes this draft from the pending queue.
              </div>
            )}

            {dialogError && !previewLoading && (
              <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive space-y-3">
                <div>{dialogError}</div>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      void refetch()
                    }}
                  >
                    Refresh list
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      setConfirmAction(null)
                      setPreview(null)
                      setDialogError(null)
                    }}
                  >
                    Close
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </ConfirmDialog>
    </div>
  )
}

function GroupedVersionList({ data, statusBadge, viewDiff, viewingDiff, diffData, acting, handleActivate, handleDiscard, handleRollback, setConfirmAction }: {
  data: ConfigVersion[]
  statusBadge: (s: string) => React.ReactNode
  viewDiff: (id: string) => void
  viewingDiff: string | null
  diffData: any
  acting: string | null
  handleActivate: (id: string) => void
  handleDiscard: (id: string) => void
  handleRollback: (id: string) => void
  setConfirmAction: (action: { type: 'activateAll' } | null) => void
}) {
  const drafts = data.filter(v => v.status === 'Draft')
  const active = data.filter(v => v.status === 'Active')
  const history = data.filter(v => v.status === 'Superseded')

  const handleActivateAll = () => {
    setConfirmAction({ type: 'activateAll' })
  }

  const renderVersion = (v: ConfigVersion) => (
    <div key={v.id} className="border rounded-lg">
      <div className="flex items-center gap-4 px-4 py-3">
        <div className="font-mono text-sm font-bold">v{v.version}</div>
        {statusBadge(v.status)}
        <div className="flex-1">
          <div className="text-sm font-medium">{v.label}</div>
          <div className="text-xs text-muted-foreground">
            by {v.created_by} &middot; {new Date(v.created_at).toLocaleString()}
          </div>
        </div>
        <div className="flex gap-1">
          <Button variant="ghost" size="sm" onClick={() => viewDiff(v.id)}>
            <Eye className="h-4 w-4 mr-1" /> {viewingDiff === v.id ? 'Hide' : 'View'}
          </Button>
          {v.status === 'Draft' && (
            <>
              <Button variant="ghost" size="sm" onClick={() => handleActivate(v.id)} disabled={acting === v.id}>
                <Play className="h-4 w-4 mr-1" /> Activate
              </Button>
              <Button variant="ghost" size="sm" onClick={() => handleDiscard(v.id)} disabled={acting === v.id}>
                <X className="h-4 w-4 mr-1" /> Discard
              </Button>
            </>
          )}
          {v.status === 'Superseded' && (
            <Button variant="ghost" size="sm" onClick={() => handleRollback(v.id)} disabled={acting === v.id}>
              <RotateCcw className="h-4 w-4 mr-1" /> Rollback
            </Button>
          )}
        </div>
      </div>

      {viewingDiff === v.id && diffData && (
        <div className="border-t px-4 py-3 bg-muted/30">
          {diffData.changes?.map((c: any, i: number) => {
            const colors: Record<string, string> = {
              doctype_added: 'text-green-700',
              field_added: 'text-green-700',
              constraint_added: 'text-green-700',
              doctype_removed: 'text-red-700',
              field_removed: 'text-red-700',
              constraint_removed: 'text-amber-700',
              field_type_changed: 'text-red-700',
              field_renamed: 'text-blue-700',
              field_required_changed: 'text-amber-700',
            }
            return (
              <div key={i} className={`text-sm py-1 ${colors[c.type] || ''}`}>
                {c.breaking && '🔴 '}
                {c.message}
              </div>
            )
          })}
          {(!diffData.changes || diffData.changes.length === 0) && (
            <div className="text-sm text-muted-foreground">No changes detected.</div>
          )}
        </div>
      )}
    </div>
  )

  return (
    <div className="space-y-6">
      {drafts.length > 0 && (
        <div>
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-sm font-semibold uppercase tracking-wider text-amber-600 flex items-center gap-2">
              <span className="h-2 w-2 rounded-full bg-amber-500 inline-block" />
              Pending Activation ({drafts.length})
            </h2>
            {drafts.length > 1 && (
              <Button variant="outline" size="sm" onClick={handleActivateAll} className="text-xs">
                <Play className="h-3 w-3 mr-1" /> Activate All
              </Button>
            )}
          </div>
          <div className="space-y-2">
            {drafts.map(renderVersion)}
          </div>
        </div>
      )}

      {active.length > 0 && (
        <div>
          <h2 className="text-sm font-semibold uppercase tracking-wider text-green-600 flex items-center gap-2 mb-2">
            <span className="h-2 w-2 rounded-full bg-green-500 inline-block" />
            Active ({active.length})
          </h2>
          <div className="space-y-2">
            {active.map(renderVersion)}
          </div>
        </div>
      )}

      {history.length > 0 && (
        <div>
          <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-2 mb-2">
            <span className="h-2 w-2 rounded-full bg-muted-foreground/30 inline-block" />
            History ({history.length})
          </h2>
          <div className="space-y-2">
            {history.map(renderVersion)}
          </div>
        </div>
      )}
    </div>
  )
}

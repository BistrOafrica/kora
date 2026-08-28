import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { deletePageManifest, fetchPageManifests, type PageManifestListEntry } from '@/lib/api/page-manifests'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Plus, Trash2, Eye, ExternalLink } from 'lucide-react'
import { toast } from '@/components/ui/Toast'
import { useNavigate } from '@tanstack/react-router'
import { sitePath } from '@/lib/basepath'
import { manifestRouteToPageSegment } from '@/manifest/runtime/policy'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { HelpTooltip } from '@/components/ui/help-tooltip'

export default function AdminPageManifestsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [deleting, setDeleting] = useState<string | null>(null)

  const { data: manifests, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['page-manifests'],
    queryFn: fetchPageManifests,
    staleTime: 30_000,
  })

  const handleDelete = async (name: string) => {
    setDeleting(name)
    try {
      await deletePageManifest(name)
      queryClient.invalidateQueries({ queryKey: ['page-manifests'] })
      toast('success', `Page manifest "${name}" deleted`)
    } catch (err: any) {
      toast('error', err.message || 'Failed to delete page manifest')
    } finally { setDeleting(null) }
  }

  if (isLoading) return <div className="p-8 space-y-4"><Skeleton className="h-8 w-48" /><Skeleton className="h-64 w-full" /></div>

  return (
    <div className="p-4 md:p-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold">
            Page manifests
            <HelpTooltip label="Page manifests help">Manage RFC-native screens rendered by the manifest runtime.</HelpTooltip>
          </h1>
        </div>
        <Button onClick={() => navigate({ to: '/workspace/admin/page-manifests/new' })}><Plus className="mr-2 h-4 w-4" />New page</Button>
      </div>
      {isError ? (
        <div className="rounded-lg border border-dashed p-12 text-center">
          <p className="font-medium">We couldn’t load your page manifests.</p>
          <p className="mt-1 text-sm text-muted-foreground">{error instanceof Error ? error.message : 'Check your connection and try again.'}</p>
          <Button variant="outline" size="sm" className="mt-4" onClick={() => refetch()}>Try again</Button>
        </div>
      ) : !manifests?.length ? (
        <div className="rounded-lg border border-dashed p-12 text-center">
          <p className="font-medium">No page manifests yet</p>
          <p className="mt-1 text-sm text-muted-foreground">Create a manifest-backed screen for your team.</p>
          <Button size="sm" className="mt-4" onClick={() => navigate({ to: '/workspace/admin/page-manifests/new' })}><Plus className="mr-2 h-4 w-4" />Create your first page</Button>
        </div>
      ) : (
        <div className="rounded-lg border">
          <Table>
            <TableHeader><TableRow><TableHead>Page</TableHead><TableHead>Route</TableHead><TableHead>Status</TableHead><TableHead>Layout</TableHead><TableHead>Package</TableHead><TableHead className="w-24">Actions</TableHead></TableRow></TableHeader>
            <TableBody>
              {manifests.map((v: PageManifestListEntry) => (
                <TableRow key={v.name}>
                  <TableCell className="font-medium">{v.label || v.name}</TableCell>
                  <TableCell className="font-mono text-xs">{v.route}</TableCell>
                  <TableCell><Badge variant="outline">{v.status}</Badge></TableCell>
                  <TableCell className="text-xs text-muted-foreground">{v.layout}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{v.module}</TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <Button variant="ghost" size="icon" className="h-8 w-8" aria-label={`Edit ${v.label || v.name}`} title="Edit page manifest" onClick={() => navigate({ to: '/workspace/admin/page-manifests/$name', params: { name: v.name } })}><Eye className="h-4 w-4" /></Button>
                      <a className="inline-flex size-8 items-center justify-center rounded-lg text-sm transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring" href={sitePath(`/workspace/${encodeURIComponent(manifestRouteToPageSegment(v.route))}`)} target="_blank" rel="noopener" aria-label={`Open ${v.label || v.name}`} title="Open page"><ExternalLink className="h-4 w-4" /></a>
                      <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" aria-label={`Delete ${v.label || v.name}`} title="Delete page manifest" disabled={deleting === v.name} onClick={() => handleDelete(v.name)}><Trash2 className="h-4 w-4" /></Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}

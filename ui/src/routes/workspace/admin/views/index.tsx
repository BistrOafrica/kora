import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { fetchViews, deleteView, type ViewListEntry } from '@/lib/api/views'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Plus, Trash2, Eye, ExternalLink } from 'lucide-react'
import { toast } from '@/components/ui/Toast'
import { useNavigate } from '@tanstack/react-router'
import { sitePath } from '@/lib/basepath'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'

export default function AdminViewsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [deleting, setDeleting] = useState<string | null>(null)

  const { data: views, isLoading } = useQuery({
    queryKey: ['views'],
    queryFn: fetchViews,
    staleTime: 30_000,
  })

  const handleDelete = async (name: string) => {
    setDeleting(name)
    try {
      await deleteView(name)
      queryClient.invalidateQueries({ queryKey: ['views'] })
      toast('success', `View "${name}" deleted`)
    } catch (err: any) {
      toast('error', err.message || 'Failed to delete view')
    } finally { setDeleting(null) }
  }

  if (isLoading) return <div className="p-8 space-y-4"><Skeleton className="h-8 w-48" /><Skeleton className="h-64 w-full" /></div>

  return (
    <div className="p-4 md:p-8">
      <div className="mb-6 flex items-center justify-between">
        <div><h1 className="text-2xl font-bold">Views</h1><p className="text-sm text-muted-foreground mt-1">Manage application screens</p></div>
        <Button onClick={() => navigate({ to: '/workspace/admin/views/new' })}><Plus className="mr-2 h-4 w-4" />New View</Button>
      </div>
      {!views?.length ? (
        <div className="rounded-lg border p-12 text-center text-muted-foreground">No views configured yet.</div>
      ) : (
        <div className="rounded-lg border">
          <Table>
            <TableHeader><TableRow><TableHead>Name</TableHead><TableHead>Route</TableHead><TableHead>Type</TableHead><TableHead>Layout</TableHead><TableHead>Module</TableHead><TableHead className="w-24">Actions</TableHead></TableRow></TableHeader>
            <TableBody>
              {views.map((v: ViewListEntry) => (
                <TableRow key={v.name}>
                  <TableCell className="font-medium">{v.label || v.name}</TableCell>
                  <TableCell className="font-mono text-xs">{v.route}</TableCell>
                  <TableCell><Badge variant="outline">{v.type}</Badge></TableCell>
                  <TableCell className="text-xs text-muted-foreground">{v.layout}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{v.module}</TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => navigate({ to: '/workspace/admin/views/$name', params: { name: v.name } })}><Eye className="h-4 w-4" /></Button>
                      <a href={sitePath(`/workspace/pages/${encodeURIComponent(v.route)}`)} target="_blank" rel="noopener"><Button variant="ghost" size="icon" className="h-8 w-8"><ExternalLink className="h-4 w-4" /></Button></a>
                      <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" disabled={deleting === v.name} onClick={() => handleDelete(v.name)}><Trash2 className="h-4 w-4" /></Button>
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

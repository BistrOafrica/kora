import { useEffect, useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { fetchViewByName, createView, updateView } from '@/lib/api/views'
import { useBuilderStore } from '@/lib/builder-store'
import { Palette } from '@/components/views/builder/Palette'
import { Canvas } from '@/components/views/builder/Canvas'
import { Inspector } from '@/components/views/builder/Inspector'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { toast } from '@/components/ui/Toast'
import { sitePath } from '@/lib/basepath'
import { Save, Eye, Undo, Redo, RotateCcw } from 'lucide-react'

const VIEW_TYPES = ['workspace', 'dashboard', 'collection', 'register', 'form', 'custom']
const LAYOUTS = ['single', 'two_panel', 'three_panel', 'grid']

export default function ViewEditorPage() {
  const params = useParams({ strict: false }) as { name?: string }
  const name = params.name
  const navigate = useNavigate()
  const store = useBuilderStore()
  const [saving, setSaving] = useState(false)

  const isNew = !name

  // Load existing view.
  const { data: existingView, isLoading, error } = useQuery({
    queryKey: ['view', name],
    queryFn: () => fetchViewByName(name!),
    enabled: !!name,
  })

  useEffect(() => {
    if (existingView) {
      store.loadView(existingView)
    } else if (isNew) {
      store.initNew('workspace', 'single')
    }
  }, [existingView, isNew])

  const handleSave = async () => {
    if (!store.working) return
    setSaving(true)
    try {
      const viewData = store.working.view
      if (isNew) {
        const result = await createView(viewData)
        toast('success', `Draft version ${result.version_num} created`)
        navigate({ to: '/workspace/admin/views/$name', params: { name: viewData.name } })
      } else {
        const result = await updateView(name!, viewData)
        toast('success', `Draft version ${result.version_num} saved`)
      }
      store.markClean()
    } catch (err: any) {
      toast('error', err.message || 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  const handlePreview = () => {
    if (!store.working) return
    const route = store.working.view.route
    if (route) {
      window.open(sitePath(`/workspace/pages/${encodeURIComponent(route)}?version=draft`), '_blank')
    }
  }

  // Keyboard shortcuts.
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Delete' || e.key === 'Backspace') {
        if (store.selectedComponentId && document.activeElement?.tagName !== 'INPUT') {
          store.removeComponent(store.selectedComponentId)
        }
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'z') {
        e.preventDefault()
        if (e.shiftKey) store.redo()
        else store.undo()
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault()
        handleSave()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [store.selectedComponentId])

  const workingView = store.working?.view
  const compCount = workingView?.components?.length || 0

  if (!workingView) {
    return (
      <div className="flex h-[calc(100vh-3rem)] items-center justify-center text-sm text-muted-foreground">
        {error ? 'Failed to load view.' : isLoading ? 'Loading view...' : 'Preparing view editor...'}
      </div>
    )
  }

  return (
    <div className="flex flex-col h-[calc(100vh-3rem)]">
      {/* Toolbar */}
      <div className="flex items-center gap-3 px-4 py-2 border-b bg-background shrink-0">
        <Input
          value={workingView.name || ''}
          onChange={(e) => store.updateMeta({ name: e.target.value })}
          placeholder="View Name"
          className="h-8 w-40 text-sm font-semibold"
        />
        <Input
          value={workingView.route || ''}
          onChange={(e) => store.updateMeta({ route: e.target.value })}
          placeholder="/route"
          className="h-8 w-32 text-xs font-mono"
        />
        <Select value={workingView.type || 'workspace'} onValueChange={(v) => store.updateMeta({ type: v || 'workspace' })}>
          <SelectTrigger className="h-8 w-32 text-xs"><SelectValue /></SelectTrigger>
          <SelectContent>{VIEW_TYPES.map((t) => <SelectItem key={t} value={t}>{t}</SelectItem>)}</SelectContent>
        </Select>
        <Select value={workingView.layout || 'single'} onValueChange={(v) => store.updateMeta({ layout: v || 'single' })}>
          <SelectTrigger className="h-8 w-28 text-xs"><SelectValue /></SelectTrigger>
          <SelectContent>{LAYOUTS.map((l) => <SelectItem key={l} value={l}>{l}</SelectItem>)}</SelectContent>
        </Select>

        <div className="flex-1" />

        <Button variant="ghost" size="icon" className="h-8 w-8" disabled={store.undoStack.length === 0} onClick={() => store.undo()} title="Undo (Ctrl+Z)">
          <Undo className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8" disabled={store.redoStack.length === 0} onClick={() => store.redo()} title="Redo (Ctrl+Shift+Z)">
          <Redo className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8" disabled={!store.dirty} onClick={() => store.working && store.loadView(store.working)} title="Reset">
          <RotateCcw className="h-4 w-4" />
        </Button>

        <div className="h-6 w-px bg-border" />

        <Button variant="outline" size="sm" className="h-8 text-xs" onClick={handlePreview} disabled={!workingView.route}>
          <Eye className="h-3.5 w-3.5 mr-1" />Preview
        </Button>
        <Button size="sm" className="h-8 text-xs" onClick={handleSave} disabled={saving || !workingView.name}>
          <Save className="h-3.5 w-3.5 mr-1" />{saving ? 'Saving...' : 'Save Draft'}
        </Button>
      </div>

      {/* Status bar */}
      <div className="flex items-center gap-4 px-4 py-1 border-b bg-muted/10 text-[11px] text-muted-foreground shrink-0">
        <span>{compCount} component{compCount !== 1 ? 's' : ''}</span>
        {store.dirty && <span className="text-amber-600 dark:text-amber-400">● Unsaved changes</span>}
        {store.errors.length > 0 && <span className="text-destructive">⚠ {store.errors.length} error{store.errors.length !== 1 ? 's' : ''}</span>}
        {!store.dirty && !isNew && <span>✓ Saved</span>}
      </div>

      {/* Three-panel body */}
      <div className="flex-1 flex overflow-hidden">
        <div className="w-[220px] shrink-0 overflow-hidden">
          <Palette />
        </div>
        <div className="flex-1 overflow-auto">
          <Canvas />
        </div>
        <div className="w-[300px] shrink-0 overflow-hidden">
          <Inspector />
        </div>
      </div>
    </div>
  )
}

import { create } from 'zustand'
import type { ViewConfig, ViewComponent } from '@/lib/api/views'

interface BuilderError {
  componentId: string
  field?: string
  message: string
}

interface BuilderState {
  original: ViewConfig | null
  working: ViewConfig | null
  selectedComponentId: string | null
  undoStack: any[]
  redoStack: any[]
  errors: BuilderError[]
  dirty: boolean

  // Actions
  loadView: (view: ViewConfig) => void
  initNew: (type: string, layout: string) => void
  updateMeta: (patch: Partial<ViewConfig['view']>) => void
  addComponent: (region: string, type: string, position: number) => void
  addChildComponent: (parentId: string, type: string) => void
  removeComponent: (id: string) => void
  updateComponent: (id: string, patch: Partial<ViewComponent>) => void
  moveComponent: (id: string, region: string, position: number) => void
  selectComponent: (id: string | null) => void
  undo: () => void
  redo: () => void
  markClean: () => void
}

// Find a component by ID in the tree (recursive).
function findComponent(comps: ViewComponent[], id: string): ViewComponent | null {
  for (const c of comps) {
    if (c.id === id) return c
    if (c.components?.length) {
      const found = findComponent(c.components, id)
      if (found) return found
    }
  }
  return null
}

// Update a component by ID in the tree (recursive, returns new array).
function updateInTree(comps: ViewComponent[], id: string, patch: Partial<ViewComponent>): ViewComponent[] {
  return comps.map((c) => {
    if (c.id === id) return { ...c, ...patch }
    if (c.components?.length) {
      return { ...c, components: updateInTree(c.components, id, patch) }
    }
    return c
  })
}


function addChildToTree(comps: ViewComponent[], parentId: string, child: ViewComponent): ViewComponent[] {
  return comps.map((c) => {
    if (c.id === parentId) {
      return { ...c, components: [...(c.components || []), child] }
    }
    if (c.components?.length) {
      return { ...c, components: addChildToTree(c.components, parentId, child) }
    }
    return c
  })
}

// Remove a component by ID from the tree.
function removeFromTree(comps: ViewComponent[], id: string): ViewComponent[] {
  return comps.filter((c) => c.id !== id).map((c) => {
    if (c.components?.length) {
      return { ...c, components: removeFromTree(c.components, id) }
    }
    return c
  })
}

function normalizeViewConfig(view: any): ViewConfig {
  if (view?.view) {
    return {
      ...view,
      view: { ...view.view, components: view.view.components || [] },
      is_public: !!view.is_public,
    }
  }

  return {
    view: { ...view, components: view?.components || [] },
    is_public: !!view?.public_access?.enabled,
  }
}

function hasWorkingView(working: ViewConfig | null): working is ViewConfig {
  return !!working?.view
}

function pushUndo(state: BuilderState): Pick<BuilderState, 'undoStack' | 'redoStack' | 'dirty'> {
  return {
    undoStack: [...state.undoStack.slice(-49), state.working],
    redoStack: [],
    dirty: true,
  }
}

export const useBuilderStore = create<BuilderState>((set, get) => ({
  original: null,
  working: null,
  selectedComponentId: null,
  undoStack: [],
  redoStack: [],
  errors: [],
  dirty: false,

  loadView: (view: any) => {
    // Accept both raw View (from admin API) and wrapped ViewConfig (from runtime API).
    const wrapped = normalizeViewConfig(view)
    set({ original: wrapped, working: JSON.parse(JSON.stringify(wrapped)), dirty: false, undoStack: [], redoStack: [], errors: [] })
  },

  initNew: (type, layout) => set({
    original: null,
    working: {
      view: { name: '', route: '', type, layout, label: '', module: 'Workspace', components: [], source_doctype: '' },
      is_public: false,
    },
    dirty: false,
    undoStack: [],
    redoStack: [],
    errors: [],
  }),

  updateMeta: (patch) => set((s) => {
    if (!hasWorkingView(s.working)) return s
    return {
      ...pushUndo(s),
      working: { ...s.working, view: { ...s.working.view, ...patch } },
    }
  }),

  addComponent: (region, type, position) => set((s) => {
    if (!hasWorkingView(s.working)) return s
    const id = `comp_${Date.now()}`
    const comp: ViewComponent = { id, type, region, position, label: type.replace(/_/g, ' ') }
    const comps = [...(s.working.view.components || [])]
    comps.splice(position, 0, comp)
    return {
      ...pushUndo(s),
      working: { ...s.working, view: { ...s.working.view, components: comps } },
      selectedComponentId: id,
    }
  }),


  addChildComponent: (parentId, type) => set((s) => {
    if (!hasWorkingView(s.working)) return s
    const parent = findComponent(s.working.view.components || [], parentId)
    if (!parent) return s
    const child: ViewComponent = {
      id: `comp_${Date.now()}`,
      type,
      region: parent.region || 'main',
      position: parent.components?.length || 0,
      label: type.replace(/_/g, ' '),
    }
    return {
      ...pushUndo(s),
      working: {
        ...s.working,
        view: {
          ...s.working.view,
          components: addChildToTree(s.working.view.components || [], parentId, child),
        },
      },
      selectedComponentId: child.id,
    }
  }),

  removeComponent: (id) => set((s) => {
    if (!hasWorkingView(s.working)) return s
    return {
      ...pushUndo(s),
      working: { ...s.working, view: { ...s.working.view, components: removeFromTree(s.working.view.components || [], id) } },
      selectedComponentId: s.selectedComponentId === id ? null : s.selectedComponentId,
    }
  }),

  updateComponent: (id, patch) => set((s) => {
    if (!hasWorkingView(s.working)) return s
    return {
      ...pushUndo(s),
      working: { ...s.working, view: { ...s.working.view, components: updateInTree(s.working.view.components || [], id, patch) } },
    }
  }),

  moveComponent: (id, region, position) => set((s) => {
    if (!hasWorkingView(s.working)) return s
    const comp = findComponent(s.working.view.components || [], id)
    if (!comp) return s
    return {
      ...pushUndo(s),
      working: {
        ...s.working,
        view: {
          ...s.working.view,
          components: [
            ...removeFromTree(s.working.view.components || [], id),
            { ...comp, region, position },
          ].sort((a, b) => a.position - b.position),
        },
      },
    }
  }),

  selectComponent: (id) => set({ selectedComponentId: id }),
  undo: () => set((s) => {
    if (s.undoStack.length === 0) return s
    const prev = s.undoStack[s.undoStack.length - 1]
    return {
      working: prev ? normalizeViewConfig(prev) : null,
      undoStack: s.undoStack.slice(0, -1),
      redoStack: [...s.redoStack, s.working],
      dirty: true,
    }
  }),
  redo: () => set((s) => {
    if (s.redoStack.length === 0) return s
    const next = s.redoStack[s.redoStack.length - 1]
    return {
      working: next ? normalizeViewConfig(next) : null,
      redoStack: s.redoStack.slice(0, -1),
      undoStack: [...s.undoStack, s.working],
      dirty: true,
    }
  }),
  markClean: () => set({ dirty: false }),
}))

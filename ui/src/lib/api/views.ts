import { api } from './client'
import type { DocTypeSchema } from '@/types/kora'

// View types — mirrors Go doctype.View
export interface ViewComponent {
  id: string
  type: string
  region: string
  label?: string
  source_doctype?: string
  bindings?: Record<string, string>
  filters?: ViewFilter[]
  actions?: ViewAction[]
  rules?: ViewRule[]
  components?: ViewComponent[]
  desktop_columns?: string[]
  mobile_columns?: string[]
  position: number
  span?: number
}

export interface ViewAction {
  id: string
  trigger: string
  type: string
  config?: Record<string, any>
}

export interface ViewRule {
  target: string
  condition: ViewCondition
}

export interface ViewCondition {
  field: string
  op: string
  value: any
}

export interface ViewFilter {
  field: string
  op: string
  value: any
}

export interface ViewConfig {
  view: {
    name: string
    route: string
    type: string
    layout: string
    label: string
    module: string
    source_doctype?: string
    components: ViewComponent[]
    public_access?: {
      enabled: boolean
      components: string[]
      allow_mutations: boolean
    }
  }
  components?: ViewComponent[]
  is_public: boolean
}

export interface ViewListEntry {
  name: string
  route: string
  type: string
  layout: string
  label: string
  module: string
}

// Fetch a view config by route (authenticated).
export async function fetchViewByRoute(route: string, version?: string): Promise<ViewConfig> {
  const params = new URLSearchParams({ route })
  if (version) params.set('version', version)
  return api.get<ViewConfig>(`/api/v1/views?${params.toString()}`)
}

// Fetch a view config by name (admin).
export async function fetchViewByName(name: string): Promise<ViewConfig> {
  return api.get<ViewConfig>(`/api/v1/system/views/${encodeURIComponent(name)}`)
}

// List all views for the site.
export async function fetchViews(): Promise<ViewListEntry[]> {
  return api.get<ViewListEntry[]>('/api/v1/system/views')
}

// Create a view.
export async function createView(data: any): Promise<any> {
  return api.post('/api/v1/system/views', data)
}

// Update a view.
export async function updateView(name: string, data: any): Promise<any> {
  return api.put(`/api/v1/system/views/${encodeURIComponent(name)}`, data)
}

// Delete a view.
export async function deleteView(name: string): Promise<any> {
  return api.delete(`/api/v1/system/views/${encodeURIComponent(name)}`)
}

// Validate a view config.
export async function validateView(data: any): Promise<{ valid: boolean; errors?: { message: string }[] }> {
  return api.post('/api/v1/system/views/validate', data)
}

// Execute a view action (server-side).
export async function executeViewAction(actionId: string, view: string, component: string, context: Record<string, any>): Promise<any> {
  return api.post(`/api/v1/view/action/${encodeURIComponent(actionId)}`, {
    view,
    component,
    context,
  })
}

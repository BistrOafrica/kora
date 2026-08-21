import { renderToStaticMarkup } from 'react-dom/server'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, expect, it } from 'vitest'
import type { PageManifest } from '@/manifest/schema/page'
import { createSimulatedResourceState, ManifestRenderer } from './ManifestRenderer'

function manifest(overrides: Partial<PageManifest> = {}): PageManifest {
  const base: PageManifest = {
    apiVersion: 'ui.kora.dev/v1',
    kind: 'Page',
    metadata: {
      name: 'orders',
      version: '1.0.0',
      package: 'tenant.ops',
      status: 'draft',
    },
    spec: {
      route: '/orders',
      runtime: '>=2.0.0 <3.0.0',
      permissions: [],
      capabilities: ['tables'],
      offline: 'read_only',
      resources: [
        { id: 'orders', query: 'document.list', params: { doctype: 'Sales Order' } },
      ],
      actions: [],
      layout: {
        type: 'single',
        columns: 12,
        children: [
          {
            id: 'orders_table',
            component: 'record_table',
            version: 1,
            region: 'main',
            position: 0,
            props: { title: 'Orders', desktop_columns: ['name', 'status'] },
            data: 'orders.data',
            required_capabilities: ['tables'],
          },
        ],
      },
    },
  }

  return {
    ...base,
    ...overrides,
    metadata: { ...base.metadata, ...overrides.metadata },
    spec: { ...base.spec, ...overrides.spec },
  }
}

function renderManifest(page: PageManifest, mode: 'editor' | 'preview' | 'runtime' = 'preview') {
  const client = new QueryClient()
  return renderToStaticMarkup(
    <QueryClientProvider client={client}>
      <ManifestRenderer
        manifest={page}
        mode={mode}
        selectedComponentId="orders_table"
        resourceState={createSimulatedResourceState(page, 'normal')}
      />
    </QueryClientProvider>,
  )
}

describe('ManifestRenderer', () => {
  it('resolves PageComponent types through the registered Kora component registry', () => {
    const markup = renderManifest(manifest())

    expect(markup).toContain('data-manifest-render-mode="preview"')
    expect(markup).toContain('Sales Order-001')
    expect(markup).toContain('aria-label="Orders"')
  })

  it('renders unsupported components through a typed fallback', () => {
    const page = manifest({
      spec: {
        ...manifest().spec,
        layout: {
          ...manifest().spec.layout,
          children: [
            {
              id: 'unknown',
              component: 'not_registered',
              version: 1,
              region: 'main',
              position: 0,
              props: { title: 'Unknown' },
            },
          ],
        },
      },
    })

    expect(renderManifest(page)).toContain('Unsupported component type: not_registered')
  })

  it('wraps real components with selection chrome only in editor mode', () => {
    const markup = renderManifest(manifest(), 'editor')

    expect(markup).toContain('data-component-id="orders_table"')
    expect(markup).toContain('aria-pressed="true"')
    expect(markup).toContain('Duplicate')
    expect(markup).toContain('Remove')
    expect(markup).toContain('Sales Order-001')
  })

  it('renders preview mode without editor chrome', () => {
    const markup = renderManifest(manifest(), 'preview')

    expect(markup).toContain('data-component-id="orders_table"')
    expect(markup).not.toContain('aria-pressed')
    expect(markup).not.toContain('Duplicate')
    expect(markup).not.toContain('Remove')
  })

  it('valid source edits rerender through the same manifest runtime path', () => {
    const source = JSON.stringify({
      ...manifest(),
      spec: {
        ...manifest().spec,
        layout: {
          ...manifest().spec.layout,
          children: [
            {
              ...manifest().spec.layout.children[0],
              props: { title: 'Edited orders', desktop_columns: ['name'] },
            },
          ],
        },
      },
    })
    const parsed = JSON.parse(source) as PageManifest

    const markup = renderManifest(parsed)

    expect(markup).toContain('aria-label="Edited orders"')
    expect(markup).toContain('Sales Order-001')
  })
})

import { renderToStaticMarkup } from 'react-dom/server'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, expect, it, vi } from 'vitest'
import type { PageManifest } from '@/manifest/schema/page'

vi.mock('../../lib/page-runtime', () => ({
  usePageManifest: vi.fn(),
}))

vi.mock('@tanstack/react-router', () => ({
  useSearch: () => ({}),
}))

vi.mock('./ManifestRenderer', () => ({
  ManifestRenderer: ({ manifest }: { manifest: PageManifest }) => (
    <div data-testid="manifest-renderer">{manifest.metadata.name}</div>
  ),
}))

vi.mock('@/components/ui/skeleton', () => ({
  Skeleton: ({ className }: { className?: string }) => <div data-testid="skeleton" className={className} />,
}))

const { usePageManifest } = await import('../../lib/page-runtime')
const { ManifestRouteRenderer } = await import('./ManifestRouteRenderer')

function manifest(overrides: Partial<PageManifest> = {}): PageManifest {
  const base: PageManifest = {
    apiVersion: 'ui.kora.dev/v1',
    kind: 'Page',
    metadata: {
      name: 'sales-orders',
      version: '1.0.0',
      package: 'tenant.ops',
      status: 'active',
    },
    spec: {
      route: '/sales-orders',
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

function render(route: string) {
  const client = new QueryClient()
  return renderToStaticMarkup(
    <QueryClientProvider client={client}>
      <ManifestRouteRenderer route={route} />
    </QueryClientProvider>,
  )
}

describe('ManifestRouteRenderer', () => {
  it('renders an active manifest route when the builder output is valid', () => {
    vi.mocked(usePageManifest).mockReturnValue({
      data: manifest(),
      isLoading: false,
      isError: false,
      error: null,
    } as any)

    const markup = render('/sales-orders')

    expect(markup).toContain('sales-orders')
    expect(markup).toContain('data-testid="manifest-renderer"')
  })

  it('fails closed on an invalid manifest before rendering the active route', () => {
    const broken = manifest()
    broken.spec.layout.children[0].data = undefined
    vi.mocked(usePageManifest).mockReturnValue({
      data: broken,
      isLoading: false,
      isError: false,
      error: null,
    } as any)

    const markup = render('/sales-orders')

    expect(markup).toContain('Screen needs attention')
    expect(markup).toContain('record_table needs a data resource binding.')
  })

  it('renders the not-found state when no page manifest is available', () => {
    vi.mocked(usePageManifest).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error('missing'),
    } as any)

    const markup = render('/missing')

    expect(markup).toContain('Screen not found')
    expect(markup).toContain('missing')
  })
})

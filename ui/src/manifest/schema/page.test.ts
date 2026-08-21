import { describe, expect, it } from 'vitest'
import { PAGE_COMPONENT_LIBRARY, createBlankPageManifest, validatePageManifestContract, type PageManifest } from './page'

function validManifest(): PageManifest {
  return {
    ...createBlankPageManifest(),
    metadata: {
      name: 'sales-orders',
      version: '1.0.0',
      package: 'tenant.ops',
      status: 'draft',
    },
    spec: {
      ...createBlankPageManifest().spec,
      route: '/sales-orders',
      resources: [
        { id: 'orders', query: 'document.list', params: { doctype: 'Sales Order' } },
      ],
      actions: [
        { id: 'submit_order', command: 'document.submit', input: {}, invalidate: ['orders'] },
      ],
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
            props: { title: 'Orders' },
            data: 'orders.data',
            actions: ['submit_order'],
          },
        ],
      },
    },
  }
}

describe('PageManifest validation', () => {
  it('keeps the generic page palette limited to routed data-backed screen components', () => {
    const components = PAGE_COMPONENT_LIBRARY.map((entry) => entry.component)

    expect(components).toEqual([
      'dashboard_grid',
      'metric_card',
      'chart',
      'search_box',
      'filter_bar',
      'record_table',
      'record_list',
      'record_cards',
      'record_form',
      'record_detail',
      'approval_queue',
      'kanban_board',
      'calendar_view',
    ])
    expect(components).not.toEqual(expect.arrayContaining([
      'workspace_dashboard',
      'product_grid',
      'cart_panel',
      'payment_panel',
      'scanner_input',
      'wizard',
      'public_form',
    ]))
  })

  it('accepts a valid RFC page manifest', () => {
    expect(validatePageManifestContract(validManifest())).toEqual([])
  })

  it('reports exact manifest paths for invalid resource, action, and component references', () => {
    const manifest = validManifest()
    manifest.spec.resources = []
    manifest.spec.actions[0].invalidate = ['missing_resource']
    manifest.spec.layout.children[0].data = 'missing_resource.data'
    manifest.spec.layout.children[0].actions = ['missing_action']

    const issues = validatePageManifestContract(manifest)

    expect(issues).toEqual(expect.arrayContaining([
      expect.objectContaining({ path: 'spec.actions.submit_order.invalidate' }),
      expect.objectContaining({ path: 'spec.layout.children.0.data' }),
      expect.objectContaining({ path: 'spec.layout.children.0.actions' }),
    ]))
  })

  it('rejects malformed top-level manifest contract fields', () => {
    const manifest = validManifest()
    manifest.apiVersion = 'bad' as PageManifest['apiVersion']
    manifest.kind = 'Widget' as PageManifest['kind']
    manifest.spec.route = 'workspace/pages/orders'

    expect(validatePageManifestContract(manifest)).toEqual(expect.arrayContaining([
      { path: 'apiVersion', message: 'Use ui.kora.dev/v1.' },
      { path: 'kind', message: 'Use kind Page.' },
      { path: 'spec.route', message: 'Route must start with /.' },
    ]))
  })

  it('rejects reserved routes that would shadow Kora core routes', () => {
    const manifest = validManifest()
    manifest.spec.route = '/workspace/admin/page-manifests'

    expect(validatePageManifestContract(manifest)).toEqual(expect.arrayContaining([
      { path: 'spec.route', message: 'Route conflicts with a reserved Kora route.' },
    ]))
  })

  it('rejects unsupported resource queries and action commands', () => {
    const manifest = validManifest()
    manifest.spec.resources[0].query = 'sql.raw'
    manifest.spec.actions[0].command = 'javascript.eval'

    expect(validatePageManifestContract(manifest)).toEqual(expect.arrayContaining([
      { path: 'spec.resources.orders.query', message: 'Unsupported resource query sql.raw.' },
      { path: 'spec.actions.submit_order.command', message: 'Unsupported action command javascript.eval.' },
    ]))
  })

  it('rejects executable-looking binding paths', () => {
    const manifest = validManifest()
    manifest.spec.layout.children[0].data = 'orders.data;alert(1)'
    manifest.spec.layout.children[0].props = {
      bindings: {
        title: 'name, window.location',
        total: 'amount + tax',
      },
    }

    expect(validatePageManifestContract(manifest)).toEqual(expect.arrayContaining([
      { path: 'spec.layout.children.0.data', message: 'Data binding must be an allowlisted dotted path.' },
      { path: 'spec.layout.children.0.props.bindings.total', message: 'Binding values must be allowlisted dotted paths.' },
    ]))
  })

  it('rejects data components that are not bound to a resource', () => {
    const manifest = validManifest()
    manifest.spec.layout.children[0].data = undefined

    expect(validatePageManifestContract(manifest)).toEqual(expect.arrayContaining([
      { path: 'spec.layout.children.0.data', message: 'record_table needs a data resource binding.' },
    ]))
  })

  it('rejects component doctypes that disagree with the bound resource doctype', () => {
    const manifest = validManifest()
    manifest.spec.layout.children[0].props.source_doctype = 'Customer'

    expect(validatePageManifestContract(manifest)).toEqual(expect.arrayContaining([
      { path: 'spec.layout.children.0.props.source_doctype', message: 'Component source_doctype must match resource doctype Sales Order.' },
    ]))
  })
})

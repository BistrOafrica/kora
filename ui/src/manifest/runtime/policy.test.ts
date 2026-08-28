import { describe, expect, it } from 'vitest'
import {
  isAllowedActionCommand,
  isAllowedResourceQuery,
  isReservedManifestRoute,
  isSafeBindingPath,
  isSafeManifestRoute,
  manifestRouteToPageSegment,
  normalizeManifestRoute,
} from './policy'

describe('manifest runtime policy', () => {
  it('normalizes backend manifest routes without hardcoding the workspace basepath', () => {
    expect(normalizeManifestRoute('pos')).toBe('/pos')
    expect(manifestRouteToPageSegment('/pos')).toBe('pos')
  })

  it('blocks routes that collide with Kora core namespaces', () => {
    expect(isReservedManifestRoute('/workspace/admin/page-manifests')).toBe(true)
    expect(isReservedManifestRoute('/pos')).toBe(false)
    expect(isSafeManifestRoute('/pos')).toBe(true)
    expect(isSafeManifestRoute('/pos?debug=true')).toBe(false)
  })

  it('allowlists runtime resource queries and action commands', () => {
    expect(isAllowedResourceQuery('document.list')).toBe(true)
    expect(isAllowedResourceQuery('sql.raw')).toBe(false)
    expect(isAllowedActionCommand('document.create')).toBe(true)
    expect(isAllowedActionCommand('javascript.eval')).toBe(false)
  })

  it('treats bindings as data paths, not executable expressions', () => {
    expect(isSafeBindingPath('orders.data')).toBe(true)
    expect(isSafeBindingPath('customer.address.city')).toBe(true)
    expect(isSafeBindingPath('orders.data[0]')).toBe(false)
    expect(isSafeBindingPath('orders.data;alert(1)')).toBe(false)
  })
})

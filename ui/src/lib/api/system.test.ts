import { describe, expect, it, vi } from 'vitest'

vi.mock('./client', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

import {
  fetchRollbackVersionPreview,
  rollbackVersion,
  isImmutableConfigVersion,
  selectRollbackTargetVersion,
} from './system'

describe('config version rollback helpers', () => {
  it('treats active and superseded versions as immutable', () => {
    expect(isImmutableConfigVersion({ status: 'Active' })).toBe(true)
    expect(isImmutableConfigVersion({ status: 'Superseded' })).toBe(true)
    expect(isImmutableConfigVersion({ status: 'Draft' })).toBe(false)
  })

  it('selects the newest immutable version for rollback', () => {
    const target = selectRollbackTargetVersion([
      { id: '1', site: 's', version: 1, created_at: '2026-08-13T00:00:00Z', created_by: 'a', label: 'v1', status: 'Draft' },
      { id: '2', site: 's', version: 2, created_at: '2026-08-13T00:00:00Z', created_by: 'a', label: 'v2', status: 'Active' },
      { id: '3', site: 's', version: 3, created_at: '2026-08-13T00:00:00Z', created_by: 'a', label: 'v3', status: 'Superseded' },
    ])

    expect(target?.id).toBe('3')
  })

  it('returns null when only drafts exist', () => {
    expect(selectRollbackTargetVersion([
      { id: '1', site: 's', version: 1, created_at: '2026-08-13T00:00:00Z', created_by: 'a', label: 'v1', status: 'Draft' },
    ])).toBeNull()
  })

  it('builds rollback preview and rollback endpoints for config versions', async () => {
    const { api } = await import('./client')
    vi.mocked(api.get).mockResolvedValueOnce({ version_id: '3', status: 'ok' })
    vi.mocked(api.post).mockResolvedValueOnce({ message: 'rolled back', status: 'ok' })

    await fetchRollbackVersionPreview('3')
    await rollbackVersion('3')

    expect(api.get).toHaveBeenCalledWith('/api/v1/system/config/versions/3/rollback-preview')
    expect(api.post).toHaveBeenCalledWith('/api/v1/system/config/versions/3/rollback')
  })
})

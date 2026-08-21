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
})

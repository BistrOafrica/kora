import { describe, expect, it } from 'vitest'
import {
  canTransitionManifestStatus,
  DOCTYPE_WORKSPACE_MANIFEST,
  isActiveManifest,
  isDraftManifest,
  isPreviewManifest,
  isRetiredManifest,
  WORKSPACE_DASHBOARD_MANIFEST,
  transitionManifestStatus,
  validatePageManifest,
} from './page-manifests'

describe('page manifests', () => {
  it('validate the dashboard manifest', () => {
    expect(validatePageManifest(WORKSPACE_DASHBOARD_MANIFEST)).toEqual([])
  })

  it('validate the doctype workspace manifest', () => {
    expect(validatePageManifest(DOCTYPE_WORKSPACE_MANIFEST)).toEqual([])
  })

  it('rejects malformed manifests', () => {
    expect(validatePageManifest({
      name: '',
      route: 'workspace',
      label: '',
      module: '',
      status: 'draft',
      sections: [
        { id: '', kind: 'hero' },
        { id: 'dup', kind: 'list' },
        { id: 'dup', kind: 'empty' },
      ],
    })).toEqual(expect.arrayContaining([
      expect.objectContaining({ message: expect.stringContaining('name') }),
      expect.objectContaining({ message: expect.stringContaining('route') }),
      expect.objectContaining({ message: expect.stringContaining('label') }),
      expect.objectContaining({ message: expect.stringContaining('module') }),
      expect.objectContaining({ message: expect.stringContaining('duplicate section id') }),
    ]))
  })

  it('enforces lifecycle transitions', () => {
    expect(canTransitionManifestStatus('draft', 'preview')).toBe(true)
    expect(canTransitionManifestStatus('active', 'draft')).toBe(false)

    const preview = transitionManifestStatus(WORKSPACE_DASHBOARD_MANIFEST, 'preview')
    expect(preview.status).toBe('preview')
    expect(isPreviewManifest(preview)).toBe(true)
    expect(isDraftManifest(preview)).toBe(false)
    expect(isActiveManifest(preview)).toBe(false)
    expect(isRetiredManifest(preview)).toBe(false)
  })

  it('rejects invalid lifecycle transitions', () => {
    expect(() => transitionManifestStatus(DOCTYPE_WORKSPACE_MANIFEST, 'draft')).toThrow(
      'invalid manifest lifecycle transition',
    )
  })
})

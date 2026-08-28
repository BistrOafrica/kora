import { describe, expect, it } from 'vitest'
import { getVersionConfirmDescription, getVersionConfirmLabel, getVersionConfirmTitle } from './versions-helpers'

describe('version confirmation copy', () => {
  it('exposes rollback-specific dialog copy', () => {
    expect(getVersionConfirmTitle('rollback')).toBe('Rollback Version')
    expect(getVersionConfirmDescription('rollback', null)).toBe('This will replace the current config with the selected historical version.')
    expect(getVersionConfirmLabel('rollback')).toBe('Rollback')
  })

  it('prefers explicit dialog errors over action copy', () => {
    expect(getVersionConfirmDescription('rollback', 'Preview unavailable')).toBe('Preview unavailable')
  })
})

import { describe, expect, it } from 'vitest'
import { resolveComponentEntry } from './registry'

describe('component registry', () => {
  it('resolves supported components', () => {
    expect(resolveComponentEntry('record_table')).toBeDefined()
    expect(resolveComponentEntry('chart')).toBeDefined()
  })

  it('fails closed when required capabilities are missing', () => {
    expect(resolveComponentEntry('chart', ['dashboard'])).toBeUndefined()
    expect(resolveComponentEntry('chart', ['charts'])).toBeDefined()
  })

  it('returns no entry for unknown component types', () => {
    expect(resolveComponentEntry('not-a-real-component')).toBeUndefined()
  })
})

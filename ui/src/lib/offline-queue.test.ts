import { beforeEach, describe, expect, it, vi } from 'vitest'
import offlineQueueSource from './offline-queue.ts?raw'
import {
  clearOfflineQueue,
  enqueueOfflineItem,
  loadOfflineQueue,
  markOfflineItemStatus,
  recordOfflineConflict,
} from './offline-queue'

describe('offline queue', () => {
  beforeEach(async () => {
    vi.stubGlobal('indexedDB', undefined)
    await clearOfflineQueue()
  })

  it('queues, loads, and updates operation status without localStorage', async () => {
    const item = await enqueueOfflineItem({
      id: 'op-1',
      type: 'pos.sale',
      payload: { total: 123 },
      deviceId: 'device-1',
      baseVersion: 7,
    })

    expect(item.status).toBe('queued')
    expect(item.operationId).toBe('op-1')
    expect(await loadOfflineQueue()).toHaveLength(1)

    const updated = await markOfflineItemStatus('op-1', 'accepted')
    expect(updated[0].status).toBe('accepted')
  })

  it('records conflicts as visible queue entries', async () => {
    await enqueueOfflineItem({
      id: 'op-2',
      type: 'pos.sale',
      payload: { total: 456 },
    })

    const updated = await recordOfflineConflict('op-2', { code: 'stale_base_version', message: 'Server record changed.' })
    expect(updated[0]).toMatchObject({
      id: 'op-2',
      status: 'conflict',
      error: { code: 'stale_base_version' },
    })
  })

  it('does not use localStorage for the offline business queue', () => {
    expect(offlineQueueSource).not.toContain('localStorage')
  })
})

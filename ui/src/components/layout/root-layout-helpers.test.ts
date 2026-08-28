import { describe, expect, it } from 'vitest'
import { formatRealtimeNotificationMessage, formatRealtimeNotificationTitle, getRootRealtimeBadge } from './root-layout-helpers'

describe('root layout realtime helpers', () => {
  it('maps realtime states to the header badge copy', () => {
    expect(getRootRealtimeBadge({ state: 'connected' })).toEqual({
      label: 'Realtime connected',
      variant: 'default',
      detail: 'Live data is connected',
    })
    expect(getRootRealtimeBadge({ state: 'reconnecting' })).toEqual({
      label: 'Realtime reconnecting',
      variant: 'outline',
      detail: 'Live data is reconnecting',
    })
  })

  it('derives notification title and message fallbacks', () => {
    expect(formatRealtimeNotificationTitle({ type: 'notification', title: 'Updated' })).toBe('Updated')
    expect(formatRealtimeNotificationTitle({ type: 'notification', doc_name: 'SO-1' })).toBe('SO-1')
    expect(formatRealtimeNotificationTitle({ type: 'notification' })).toBe('notification')
    expect(formatRealtimeNotificationMessage({ message: 'Saved' })).toBe('Saved')
    expect(formatRealtimeNotificationMessage({ doctype: 'Sales Order' })).toBe('Sales Order')
    expect(formatRealtimeNotificationMessage({})).toBe('Update received')
  })
})

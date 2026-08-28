import type { RealtimeConnectionState } from '@/types/api'

export function getRootRealtimeBadge(state: RealtimeConnectionState): { label: string; variant: 'default' | 'outline' | 'secondary' | 'destructive'; detail: string } {
  switch (state.state) {
    case 'connected':
      return { label: 'Realtime connected', variant: 'default', detail: state.detail || 'Live data is connected' }
    case 'reconnecting':
      return { label: 'Realtime reconnecting', variant: 'outline', detail: state.detail || 'Live data is reconnecting' }
    case 'offline':
      return { label: 'Realtime offline', variant: 'secondary', detail: state.detail || 'Live data is offline' }
    case 'unauthorized':
      return { label: 'Realtime unavailable', variant: 'destructive', detail: state.detail || 'Live data requires authentication' }
    case 'closed':
      return { label: 'Realtime closed', variant: 'secondary', detail: state.detail || 'Live data is closed' }
    case 'degraded':
      return { label: 'Realtime degraded', variant: 'outline', detail: state.detail || 'Live data is degraded' }
    case 'connecting':
    default:
      return { label: 'Realtime connecting', variant: 'outline', detail: state.detail || 'Live data is connecting' }
  }
}

export function formatRealtimeNotificationTitle(detail: { title?: string; doc_name?: string; type: string }): string {
  return detail.title || detail.doc_name || detail.type
}

export function formatRealtimeNotificationMessage(detail: { message?: string; doctype?: string }): string {
  return detail.message || detail.doctype || 'Update received'
}

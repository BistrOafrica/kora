import { useEffect, useMemo, useState } from 'react'
import type { QueryClient } from '@tanstack/react-query'
import { loadRuntimeConfig } from './runtime-config'
import type { RealtimeConnectionState } from '@/types/api'

export interface RealtimeEvent {
  type: string
  transport?: 'websocket' | 'sse'
  site?: string
  doctype?: string
  doc_name?: string
  operation_id?: string
  correlation_id?: string
  resource?: string
  payload?: Record<string, unknown>
  occurred_at?: string
  title?: string
  message?: string
  severity?: 'info' | 'success' | 'warning' | 'error'
  action?: { label: string; href?: string }
  read?: boolean
}

export function describeRealtimeState(state: RealtimeConnectionState): { label: string; tone: 'default' | 'outline' | 'secondary' | 'destructive'; detail: string } {
  switch (state.state) {
    case 'connected':
      return {
        label: 'Live',
        tone: 'default',
        detail: state.detail || 'Live updates are connected',
      }
    case 'reconnecting':
      return {
        label: 'Reconnecting',
        tone: 'outline',
        detail: state.detail || 'Live updates are reconnecting',
      }
    case 'unauthorized':
      return {
        label: 'Unavailable',
        tone: 'destructive',
        detail: state.detail || 'Live updates require authentication',
      }
    case 'offline':
      return {
        label: 'Offline',
        tone: 'secondary',
        detail: state.detail || 'Live updates are offline',
      }
    case 'closed':
      return {
        label: 'Closed',
        tone: 'secondary',
        detail: state.detail || 'Live updates are closed',
      }
    case 'degraded':
      return {
        label: 'Degraded',
        tone: 'outline',
        detail: state.detail || 'Live updates are degraded',
      }
    case 'connecting':
    default:
      return {
        label: 'Connecting',
        tone: 'outline',
        detail: state.detail || 'Live updates are connecting',
      }
  }
}

export function useRealtimeConnection(): RealtimeConnectionState {
  const runtime = useMemo(() => loadRuntimeConfig(), [])
  const [state, setState] = useState<RealtimeConnectionState>(() => ({
    state: runtime.realtimeBaseUrl ? 'connecting' : 'offline',
  }))

  useEffect(() => {
    if (!runtime.realtimeBaseUrl) {
      setState({ state: 'offline', detail: 'No realtime endpoint configured' })
      return
    }

    let aborted = false
    let reconnectDelay = 1000
    let socket: WebSocket | null = null
    let source: EventSource | null = null
    let transport: 'websocket' | 'sse' | null = null
    let retryTimer: number | null = null

    const connect = () => {
      if (aborted) return
      setState((prev) => ({ ...prev, state: prev.state === 'connected' ? 'connected' : 'connecting' }))

      try {
        const wsUrl = runtime.realtimeBaseUrl!.replace(/^http/i, 'ws')
        socket = new WebSocket(wsUrl)
        transport = 'websocket'
        socket.onopen = () => {
          reconnectDelay = 1000
          setState({ state: 'connected', last_seen_at: new Date().toISOString(), detail: 'WebSocket connected' })
        }
        socket.onmessage = (event) => handleRealtimeMessage(event.data)
        socket.onerror = () => {
          if (socket?.readyState === WebSocket.OPEN || socket?.readyState === WebSocket.CONNECTING) {
            return
          }
          startSSEFallback()
        }
        socket.onclose = () => scheduleReconnect()
        return
      } catch {
        startSSEFallback()
      }
    }

    const scheduleReconnect = () => {
      if (aborted) return
      setState((prev) => ({
        ...prev,
        state: navigator.onLine ? 'reconnecting' : 'offline',
        detail: navigator.onLine ? 'Realtime reconnecting' : 'Browser offline',
      }))
      closeTransport()
      retryTimer = window.setTimeout(connect, reconnectDelay)
      reconnectDelay = Math.min(reconnectDelay * 2, 30_000)
    }

    const closeTransport = () => {
      socket?.close()
      socket = null
      source?.close()
      source = null
    }

    const handleRealtimeMessage = (raw: unknown) => {
      setState((prev) => ({ ...prev, state: 'connected', last_seen_at: new Date().toISOString() }))
      const data = typeof raw === 'string' ? raw : ''
      if (!data) return
      try {
        const parsed = JSON.parse(data) as RealtimeEvent
        if (parsed.type === 'notification') {
          window.dispatchEvent(new CustomEvent('kora:realtime-notification', { detail: parsed }))
        }
      } catch {
        // Ignore malformed realtime payloads.
      }
    }

    const startSSEFallback = () => {
      transport = 'sse'
      closeTransport()
      try {
        source = new EventSource(runtime.realtimeBaseUrl!)
      } catch (error) {
        setState({ state: 'offline', detail: error instanceof Error ? error.message : 'Unable to open realtime stream' })
        return
      }

      source.onopen = () => {
        reconnectDelay = 1000
        setState({ state: 'connected', last_seen_at: new Date().toISOString(), detail: 'Server-sent events connected' })
      }

      source.addEventListener('connected', (event) => handleRealtimeMessage((event as MessageEvent).data))
      source.addEventListener('heartbeat', (event) => handleRealtimeMessage((event as MessageEvent).data))
      source.onmessage = (event) => handleRealtimeMessage(event.data)

      source.onerror = () => {
        if (aborted) return
        scheduleReconnect()
      }
    }

    connect()

    const onOnline = () => setState((prev) => ({ ...prev, state: 'reconnecting', detail: 'Network restored' }))
    const onOffline = () => setState((prev) => ({ ...prev, state: 'offline', detail: 'Browser offline' }))
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)

    return () => {
      aborted = true
      if (retryTimer) window.clearTimeout(retryTimer)
      closeTransport()
      window.removeEventListener('online', onOnline)
      window.removeEventListener('offline', onOffline)
    }
  }, [runtime.realtimeBaseUrl])

  return state
}

export function extractInvalidationKey(event: RealtimeEvent): string | null {
  if (event.resource) return event.resource
  if (event.doctype) return `doctype:${event.doctype}`
  if (event.operation_id) return `operation:${event.operation_id}`
  return null
}

export function mapRealtimeInvalidation(event: RealtimeEvent): Array<{ queryKey: unknown[] }> {
  const key = extractInvalidationKey(event)
  if (!key) return []
  if (key.startsWith('doctype:')) {
    const doctype = key.slice('doctype:'.length)
    return [
      { queryKey: ['view-data', doctype, true] },
      { queryKey: ['view-data', doctype, false] },
      { queryKey: ['page-manifest', `/workspace/${doctype}`, undefined] },
      { queryKey: ['page-manifests'] },
    ]
  }
  if (key.startsWith('page:')) {
    const page = key.slice('page:'.length)
    return [{ queryKey: ['page-manifest', page, undefined] }]
  }
  if (key.startsWith('operation:')) {
    return [{ queryKey: ['operation', key.slice('operation:'.length)] }]
  }
  return [{ queryKey: [key] }]
}

export function attachRealtimeInvalidation(
  runtimeBaseUrl: string | undefined,
  queryClient: QueryClient,
  onState?: (state: RealtimeConnectionState) => void,
) {
  if (!runtimeBaseUrl) {
    onState?.({ state: 'offline', detail: 'No realtime endpoint configured' })
    return () => undefined
  }

  let aborted = false
  let reconnectDelay = 1000
  let socket: WebSocket | null = null
  let source: EventSource | null = null
  let retryTimer: number | null = null

  const connect = () => {
    if (aborted) return
    onState?.({ state: 'connecting' })
    try {
      const wsUrl = runtimeBaseUrl.replace(/^http/i, 'ws')
      socket = new WebSocket(wsUrl)
      socket.onopen = () => {
        reconnectDelay = 1000
        onState?.({ state: 'connected', last_seen_at: new Date().toISOString(), detail: 'WebSocket connected' })
      }
      socket.onmessage = (event) => handleMessage(event.data)
      socket.onerror = () => startSSEFallback()
      socket.onclose = () => scheduleReconnect()
      return
    } catch {
      startSSEFallback()
    }
  }

  const closeTransport = () => {
    socket?.close()
    socket = null
    source?.close()
    source = null
  }

  const scheduleReconnect = () => {
    if (aborted) return
    onState?.({
      state: navigator.onLine ? 'reconnecting' : 'offline',
      detail: navigator.onLine ? 'Realtime reconnecting' : 'Browser offline',
    })
    closeTransport()
    retryTimer = window.setTimeout(connect, reconnectDelay)
    reconnectDelay = Math.min(reconnectDelay * 2, 30_000)
  }

  const handleMessage = (raw: unknown) => {
    onState?.({ state: 'connected', last_seen_at: new Date().toISOString() })
    const payload = typeof raw === 'string' ? raw : ''
    if (!payload) return
    try {
      const parsed = JSON.parse(payload) as RealtimeEvent
      for (const invalidation of mapRealtimeInvalidation(parsed)) {
        void queryClient.invalidateQueries({ queryKey: invalidation.queryKey })
      }
      if (parsed.type === 'change') {
        const subject = parsed.doctype || 'Record'
        const notification: RealtimeEvent = {
          type: 'notification',
          title: `${subject} updated`,
          message: parsed.doc_name ? `${subject} ${parsed.doc_name} changed` : `${subject} changed`,
          severity: 'info',
          occurred_at: parsed.occurred_at,
          site: parsed.site,
          doctype: parsed.doctype,
          doc_name: parsed.doc_name,
          resource: parsed.resource,
          payload: parsed.payload,
          action: parsed.doctype ? { label: 'Open doctype', href: `/workspace/${encodeURIComponent(parsed.doctype)}` } : undefined,
        }
        window.dispatchEvent(new CustomEvent('kora:realtime-notification', { detail: notification }))
      }
      if (parsed.type === 'notification') {
        window.dispatchEvent(new CustomEvent('kora:realtime-notification', { detail: parsed }))
      }
    } catch {
      // Ignore malformed realtime payloads.
    }
  }

  const startSSEFallback = () => {
    closeTransport()
    try {
      source = new EventSource(runtimeBaseUrl)
    } catch (error) {
      onState?.({ state: 'offline', detail: error instanceof Error ? error.message : 'Unable to open realtime stream' })
      return
    }

    source.onopen = () => {
      reconnectDelay = 1000
      onState?.({ state: 'connected', last_seen_at: new Date().toISOString(), detail: 'Server-sent events connected' })
    }

    source.onmessage = (event) => handleMessage(event.data)

    source.onerror = () => {
      if (aborted) return
      scheduleReconnect()
    }
  }

  connect()

  const onOnline = () => onState?.({ state: 'reconnecting', detail: 'Network restored' })
  const onOffline = () => onState?.({ state: 'offline', detail: 'Browser offline' })
  window.addEventListener('online', onOnline)
  window.addEventListener('offline', onOffline)

  return () => {
    aborted = true
    if (retryTimer) window.clearTimeout(retryTimer)
    closeTransport()
    window.removeEventListener('online', onOnline)
    window.removeEventListener('offline', onOffline)
  }
}

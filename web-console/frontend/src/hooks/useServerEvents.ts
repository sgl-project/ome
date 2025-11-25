import { useEffect, useRef, useCallback } from 'react'

export interface ResourceEvent {
  type: 'add' | 'update' | 'delete'
  resource: 'models' | 'runtimes' | 'services' | 'accelerators'
  name: string
  data?: any
}

export interface UseServerEventsOptions {
  onEvent?: (event: ResourceEvent) => void
  onConnected?: () => void
  onDisconnected?: () => void
  enabled?: boolean
}

/**
 * Hook to connect to server-sent events for real-time Kubernetes resource updates
 */
export function useServerEvents(options: UseServerEventsOptions = {}) {
  const { onEvent, onConnected, onDisconnected, enabled = true } = options
  const eventSourceRef = useRef<EventSource | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const reconnectAttempts = useRef(0)
  const maxReconnectAttempts = 5

  const connect = useCallback(() => {
    if (!enabled || eventSourceRef.current) {
      return
    }

    const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
    const eventSource = new EventSource(`${apiUrl}/api/v1/events`)

    eventSource.addEventListener('connected', () => {
      console.log('[SSE] Connected to event stream')
      reconnectAttempts.current = 0
      onConnected?.()
    })

    eventSource.addEventListener('add', (e) => {
      try {
        const event: ResourceEvent = JSON.parse(e.data)
        console.log('[SSE] Resource added:', event)
        onEvent?.(event)
      } catch (error) {
        console.error('[SSE] Failed to parse add event:', error)
      }
    })

    eventSource.addEventListener('update', (e) => {
      try {
        const event: ResourceEvent = JSON.parse(e.data)
        console.log('[SSE] Resource updated:', event)
        onEvent?.(event)
      } catch (error) {
        console.error('[SSE] Failed to parse update event:', error)
      }
    })

    eventSource.addEventListener('delete', (e) => {
      try {
        const event: ResourceEvent = JSON.parse(e.data)
        console.log('[SSE] Resource deleted:', event)
        onEvent?.(event)
      } catch (error) {
        console.error('[SSE] Failed to parse delete event:', error)
      }
    })

    eventSource.addEventListener('ping', () => {
      // Keep-alive ping, no action needed
    })

    eventSource.onerror = (error) => {
      console.error('[SSE] Connection error:', error)
      eventSource.close()
      eventSourceRef.current = null
      onDisconnected?.()

      // Exponential backoff reconnection
      if (reconnectAttempts.current < maxReconnectAttempts) {
        const delay = Math.min(1000 * Math.pow(2, reconnectAttempts.current), 30000)
        console.log(
          `[SSE] Reconnecting in ${delay}ms (attempt ${reconnectAttempts.current + 1}/${maxReconnectAttempts})`
        )
        reconnectTimeoutRef.current = setTimeout(() => {
          reconnectAttempts.current++
          connect()
        }, delay)
      } else {
        console.error('[SSE] Max reconnection attempts reached')
      }
    }

    eventSourceRef.current = eventSource
  }, [enabled, onEvent, onConnected, onDisconnected])

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }

    if (eventSourceRef.current) {
      console.log('[SSE] Disconnecting from event stream')
      eventSourceRef.current.close()
      eventSourceRef.current = null
      onDisconnected?.()
    }
  }, [onDisconnected])

  useEffect(() => {
    if (enabled) {
      connect()
    } else {
      disconnect()
    }

    return () => {
      disconnect()
    }
  }, [enabled, connect, disconnect])

  return {
    isConnected: eventSourceRef.current !== null,
    reconnect: () => {
      disconnect()
      reconnectAttempts.current = 0
      connect()
    },
  }
}

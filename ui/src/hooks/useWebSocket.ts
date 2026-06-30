import { useCallback, useEffect, useRef } from 'react'
import type { SignedCertificate } from '../crypto/certificate'

export interface WsMessage {
  type: string
  [key: string]: unknown
}

interface Options {
  certificate: SignedCertificate
  onMessage: (msg: WsMessage) => void
}

const BASE_DELAY = 1000
const MAX_DELAY = 30_000

// useWebSocket opens a persistent WebSocket to /ws, sends hello on connect,
// and handles reconnection with exponential backoff.
export function useWebSocket(options: Options) {
  const { certificate, onMessage } = options
  const wsRef = useRef<WebSocket | null>(null)
  const sendQueueRef = useRef<string[]>([])
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const delay = useRef(BASE_DELAY)
  const unmounted = useRef(false)
  const onMessageRef = useRef(onMessage)
  onMessageRef.current = onMessage

  const send = useCallback((payload: object) => {
    const str = JSON.stringify(payload)
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(str)
    } else {
      sendQueueRef.current.push(str)
    }
  }, [])

  useEffect(() => {
    unmounted.current = false

    function connect() {
      if (unmounted.current) return

      const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
      const ws = new WebSocket(`${protocol}://${window.location.host}/ws`)
      wsRef.current = ws

      ws.onopen = () => {
        delay.current = BASE_DELAY
        ws.send(JSON.stringify({ type: 'hello', certificate }))
        // Drain queued messages.
        const q = sendQueueRef.current.splice(0)
        q.forEach((m) => ws.send(m))
      }

      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data as string) as WsMessage
          onMessageRef.current(msg)
        } catch {
          // ignore malformed frames
        }
      }

      ws.onclose = () => {
        if (unmounted.current) return
        reconnectTimer.current = setTimeout(() => {
          delay.current = Math.min(delay.current * 2, MAX_DELAY)
          connect()
        }, delay.current)
      }

      ws.onerror = () => ws.close()
    }

    connect()

    return () => {
      unmounted.current = true
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
    }
  }, [certificate]) // reconnect if identity changes

  return { send }
}

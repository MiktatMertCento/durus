import { useEffect, useEffectEvent, useRef, useState } from 'react'
import {
  idleState,
  type ServerMessage,
  type SessionState,
  wsUrl,
} from './types.ts'

type Action = 'start' | 'stop' | 'close' | 'advance'

export function useSession() {
  const [state, setState] = useState<SessionState>(idleState)
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState('')
  const wsRef = useRef<WebSocket | null>(null)
  const retryRef = useRef(0)

  const onMessage = useEffectEvent((raw: string) => {
    try {
      const msg = JSON.parse(raw) as ServerMessage
      if (msg.type === 'state') {
        setState(msg.state)
        setError('')
      } else if (msg.type === 'error') {
        setError(msg.message)
      }
    } catch {
      // ignore malformed frames
    }
  })

  useEffect(() => {
    let closed = false
    let timer: number | undefined

    const connect = () => {
      if (closed) return
      const ws = new WebSocket(wsUrl())
      wsRef.current = ws

      ws.onopen = () => {
        setConnected(true)
        setError('')
        retryRef.current = 0
      }

      ws.onmessage = (ev) => {
        if (typeof ev.data === 'string') onMessage(ev.data)
      }

      ws.onclose = () => {
        setConnected(false)
        wsRef.current = null
        if (closed) return
        const delay = Math.min(8000, 500 * 2 ** retryRef.current)
        retryRef.current += 1
        timer = window.setTimeout(connect, delay)
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    connect()

    return () => {
      closed = true
      window.clearTimeout(timer)
      wsRef.current?.close()
    }
  }, [])

  const send = (action: Action) => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      setError('Bağlantı yok — yeniden bağlanılıyor…')
      return
    }
    ws.send(JSON.stringify({ action }))
  }

  return {
    state,
    connected,
    error,
    start: () => send('start'),
    stop: () => send('stop'),
    close: () => send('close'),
    advance: () => send('advance'),
  }
}

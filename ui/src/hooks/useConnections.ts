import { useCallback, useState } from 'react'
import type { WsMessage } from './useWebSocket'

export interface PendingRequest {
  connectionId: string
  requester: { id: string; handle: string; name: string }
}

export function useConnections(send: (payload: object) => void) {
  // Incoming connection requests not yet responded to.
  const [pendingInbound, setPendingInbound] = useState<PendingRequest[]>([])
  // IDs of users we've sent a request to that is still pending.
  const [sentTo, setSentTo] = useState<Set<string>>(new Set())

  function sendRequest(targetId: string) {
    send({ type: 'connect_request', targetId })
    setSentTo((prev) => new Set(prev).add(targetId))
  }

  function respond(connectionId: string, accept: boolean) {
    send({ type: 'connect_respond', connectionId, accept })
    setPendingInbound((prev) => prev.filter((r) => r.connectionId !== connectionId))
  }

  const handleMessage = useCallback(
    (msg: WsMessage) => {
      if (msg.type === 'pending_connects') {
        // Batch delivered on hello — seed pending inbound list.
        const inbound = (msg.inbound as PendingRequest[]) ?? []
        setPendingInbound(inbound)
        // Note: responses (accepted/rejected) are handled by the roster update
        // the server sends after accept, so no explicit state change needed here.
      } else if (msg.type === 'connect_request') {
        // Live delivery of a new incoming request.
        const req = msg as unknown as {
          connectionId: string
          requester: { id: string; handle: string; name: string }
        }
        setPendingInbound((prev) => [
          ...prev.filter((r) => r.connectionId !== req.connectionId),
          { connectionId: req.connectionId, requester: req.requester },
        ])
      } else if (msg.type === 'connect_respond') {
        // Response to a request we sent — remove from sentTo.
        const peer = (msg as unknown as { peer: { id: string } }).peer
        if (peer?.id) {
          setSentTo((prev) => {
            const next = new Set(prev)
            next.delete(peer.id)
            return next
          })
        }
      }
      // connect_request_ack — sentTo is already updated in sendRequest, nothing to do.
    },
    [],
  )

  return { pendingInbound, sentTo, sendRequest, respond, handleMessage }
}

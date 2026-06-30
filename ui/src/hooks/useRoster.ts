import { useCallback, useState } from 'react'
import type { WsMessage } from './useWebSocket'

export interface OnlineUser {
  id: string
  name: string
  signingKey: JsonWebKey
  encryptionKey: JsonWebKey
}

// useRoster manages the live list of online users from WebSocket events.
// Returns the roster and a message handler to wire into useWebSocket.
export function useRoster(ownId: string) {
  const [roster, setRoster] = useState<Map<string, OnlineUser>>(new Map())

  const handleMessage = useCallback(
    (msg: WsMessage) => {
      if (msg.type === 'roster') {
        const users = (msg.users as OnlineUser[]) ?? []
        setRoster(new Map(users.filter((u) => u.id !== ownId).map((u) => [u.id, u])))
      } else if (msg.type === 'user_joined') {
        const user = msg.user as OnlineUser
        if (user.id !== ownId) {
          setRoster((prev) => new Map(prev).set(user.id, user))
        }
      } else if (msg.type === 'user_left') {
        const id = msg.id as string
        setRoster((prev) => {
          const next = new Map(prev)
          next.delete(id)
          return next
        })
      }
    },
    [ownId],
  )

  return { roster: Array.from(roster.values()), handleMessage }
}

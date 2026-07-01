import { useCallback, useState } from 'react'
import type { WsMessage } from './useWebSocket'

export interface KnownUser {
  id: string
  handle: string
  name: string
  signingKey: JsonWebKey
  encryptionKey: JsonWebKey
  online: boolean
}

// RosterUser shape matches what the server sends in roster / user_joined messages.
interface RosterUser {
  id: string
  handle: string
  name: string
  signingKey: JsonWebKey
  encryptionKey: JsonWebKey
  online: boolean
}

// useUsers maintains the contact list (accepted connections) driven entirely
// by WebSocket messages. The server sends a full roster on connect that
// includes both online and offline contacts, then incremental user_joined /
// user_left events for presence updates.
export function useUsers(ownId: string) {
  const [directory, setDirectory] = useState<Map<string, KnownUser>>(new Map())

  const handleMessage = useCallback(
    (msg: WsMessage) => {
      if (msg.type === 'roster') {
        // Full contact list from server — replace the directory entirely.
        const users = (msg.users as RosterUser[]) ?? []
        const next = new Map<string, KnownUser>()
        for (const u of users) {
          if (u.id !== ownId) next.set(u.id, u)
        }
        setDirectory(next)
      } else if (msg.type === 'user_joined') {
        const user = msg.user as RosterUser
        if (user.id !== ownId) {
          setDirectory((prev) => new Map(prev).set(user.id, { ...user, online: true }))
        }
      } else if (msg.type === 'user_left') {
        const id = msg.id as string
        setDirectory((prev) => {
          const existing = prev.get(id)
          if (!existing) return prev
          return new Map(prev).set(id, { ...existing, online: false })
        })
      }
    },
    [ownId],
  )

  const users: KnownUser[] = Array.from(directory.values()).sort((a, b) => {
    if (a.online !== b.online) return a.online ? -1 : 1
    return a.name.localeCompare(b.name)
  })

  return { users, handleMessage }
}

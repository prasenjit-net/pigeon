import { useCallback, useEffect, useState } from 'react'
import type { WsMessage } from './useWebSocket'
import { usersApi } from '../services/api'

export interface KnownUser {
  id: string
  name: string
  signingKey: JsonWebKey
  encryptionKey: JsonWebKey
  online: boolean
}

interface DirectoryEntry {
  id: string
  name: string
  signingKey: JsonWebKey
  encryptionKey: JsonWebKey
}

// useUsers maintains the full set of registered users — fetched once via
// REST since the server keeps every registration even after a user
// disconnects — merged with live online presence from the WebSocket. This
// lets the UI show (and message) users who aren't currently connected; the
// server queues messages to them until they reconnect.
export function useUsers(ownId: string) {
  const [directory, setDirectory] = useState<Map<string, DirectoryEntry>>(new Map())
  const [online, setOnline] = useState<Set<string>>(new Set())

  useEffect(() => {
    let cancelled = false
    usersApi
      .list()
      .then((users) => {
        if (cancelled) return
        setDirectory((prev) => {
          const next = new Map(prev)
          for (const u of users) {
            if (u.id !== ownId) next.set(u.id, u)
          }
          return next
        })
      })
      .catch((err) => console.error('useUsers: failed to load directory', err))
    return () => {
      cancelled = true
    }
  }, [ownId])

  const handleMessage = useCallback(
    (msg: WsMessage) => {
      if (msg.type === 'roster') {
        const users = (msg.users as DirectoryEntry[]) ?? []
        setDirectory((prev) => {
          const next = new Map(prev)
          for (const u of users) {
            if (u.id !== ownId) next.set(u.id, u)
          }
          return next
        })
        setOnline(new Set(users.filter((u) => u.id !== ownId).map((u) => u.id)))
      } else if (msg.type === 'user_joined') {
        const user = msg.user as DirectoryEntry
        if (user.id !== ownId) {
          setDirectory((prev) => new Map(prev).set(user.id, user))
          setOnline((prev) => new Set(prev).add(user.id))
        }
      } else if (msg.type === 'user_left') {
        const id = msg.id as string
        setOnline((prev) => {
          const next = new Set(prev)
          next.delete(id)
          return next
        })
      }
    },
    [ownId],
  )

  const users: KnownUser[] = Array.from(directory.values())
    .map((u) => ({ ...u, online: online.has(u.id) }))
    .sort((a, b) => {
      if (a.online !== b.online) return a.online ? -1 : 1
      return a.name.localeCompare(b.name)
    })

  return { users, handleMessage }
}

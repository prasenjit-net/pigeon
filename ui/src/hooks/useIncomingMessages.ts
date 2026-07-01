import { useCallback } from 'react'
import { decryptMessage } from '../crypto/encrypt'
import type { WsMessage } from './useWebSocket'
import { appendMessage } from '../store/messages'
import type { ChatMessage } from './useChat'

interface ServerPersistedMsg {
  id: string
  from: string
  to: string
  groupId?: string
  encryptedPayload: string
  timestamp: number
}

interface Options {
  ownEncPrivKeyJWK: JsonWebKey
  onMessage: (fromId: string, msg: ChatMessage) => void
}

// useIncomingMessages is the single decrypt-and-persist pathway for every
// message the server hands us: both the 'pending_messages' batch sent on
// connect and live 'message' events while connected. Persistence must not
// depend on which conversation (if any) is currently open in the UI —
// otherwise messages for unopened conversations are decrypted nowhere,
// never written to localStorage, and lost for good (the server already
// marks a live delivery as done, so it won't be redelivered later).
export function useIncomingMessages({ ownEncPrivKeyJWK, onMessage }: Options) {
  const decryptAndStore = useCallback(
    async (fromId: string, id: string, encryptedPayload: string, timestamp: number) => {
      try {
        const text = await decryptMessage(encryptedPayload, ownEncPrivKeyJWK)
        const chatMsg: ChatMessage = {
          id,
          direction: 'received',
          text,
          timestamp,
          status: 'delivered',
        }
        appendMessage(fromId, chatMsg)
        onMessage(fromId, chatMsg)
      } catch (err) {
        console.error('useIncomingMessages: decrypt failed', id, err)
      }
    },
    [ownEncPrivKeyJWK, onMessage],
  )

  const handleMessage = useCallback(
    (msg: WsMessage) => {
      if (msg.type === 'pending_messages') {
        const messages = (msg.messages as ServerPersistedMsg[]) ?? []
        for (const pm of messages) {
          if (pm.groupId) continue // handled by useGroupIncomingMessages
          decryptAndStore(pm.from, pm.id, pm.encryptedPayload, pm.timestamp)
        }
      } else if (msg.type === 'message') {
        const from = msg.from as string
        const id = msg.id as string
        const encryptedPayload = msg.encryptedPayload as string
        const timestamp = (msg.timestamp as number) || Date.now()
        decryptAndStore(from, id, encryptedPayload, timestamp)
      }
    },
    [decryptAndStore],
  )

  return { handleMessage }
}

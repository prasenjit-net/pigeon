import { useCallback } from 'react'
import { decryptMessage } from '../crypto/encrypt'
import type { WsMessage } from './useWebSocket'
import { appendMessage } from '../store/messages'

interface ServerPersistedMsg {
  id: string
  from: string
  to: string
  encryptedPayload: string
  timestamp: number
}

interface Options {
  ownEncPrivKeyJWK: JsonWebKey
  onDecrypted: (fromId: string) => void
}

// usePendingMessages handles the 'pending_messages' batch sent by the server
// on connect. Each message is decrypted and stored in localStorage, then
// onDecrypted is called so the caller can update unread counts.
export function usePendingMessages({ ownEncPrivKeyJWK, onDecrypted }: Options) {
  const handlePending = useCallback(
    (msg: WsMessage) => {
      if (msg.type !== 'pending_messages') return
      const messages = (msg.messages as ServerPersistedMsg[]) ?? []

      for (const pm of messages) {
        decryptMessage(pm.encryptedPayload, ownEncPrivKeyJWK)
          .then((plaintext) => {
            appendMessage(pm.from, {
              id: pm.id,
              direction: 'received',
              text: plaintext,
              timestamp: pm.timestamp,
              status: 'delivered',
            })
            onDecrypted(pm.from)
          })
          .catch((err) => {
            console.error('usePendingMessages: decrypt failed', pm.id, err)
          })
      }
    },
    [ownEncPrivKeyJWK, onDecrypted],
  )

  return { handlePending }
}

import { useCallback } from 'react'
import { groupDecryptMessage } from '../crypto/encrypt'
import { appendGroupMessage } from '../store/messages'
import type { WsMessage } from './useWebSocket'
import type { ChatMessage } from './useChat'
import type { PersistedMsg } from '../store/messages'

interface ServerGroupMsg {
  id: string
  groupId: string
  from: string
  encryptedPayload: string
  timestamp: number
}

interface ServerPersistedMsg {
  id: string
  from: string
  groupId?: string
  encryptedPayload: string
  timestamp: number
}

interface Options {
  ownId: string
  ownEncPrivKeyJWK: JsonWebKey
  onGroupMessage: (groupId: string, msg: ChatMessage) => void
}

// useGroupIncomingMessages is the global decrypt-and-persist pathway for every
// group message received: both live group_message events and pending_messages
// batch entries that have a groupId set. Runs regardless of which group is open.
export function useGroupIncomingMessages({ ownId, ownEncPrivKeyJWK, onGroupMessage }: Options) {
  const decryptAndStore = useCallback(
    async (groupId: string, id: string, encryptedPayload: string, timestamp: number) => {
      try {
        const text = await groupDecryptMessage(encryptedPayload, ownId, ownEncPrivKeyJWK)
        const chatMsg: ChatMessage = {
          id,
          direction: 'received',
          text,
          timestamp,
          status: 'delivered',
        }
        appendGroupMessage(groupId, chatMsg as PersistedMsg)
        onGroupMessage(groupId, chatMsg)
      } catch (err) {
        console.error('useGroupIncomingMessages: decrypt failed', id, err)
      }
    },
    [ownId, ownEncPrivKeyJWK, onGroupMessage],
  )

  const handleMessage = useCallback(
    (msg: WsMessage) => {
      if (msg.type === 'group_message') {
        const gm = msg as unknown as ServerGroupMsg
        decryptAndStore(gm.groupId, gm.id, gm.encryptedPayload, gm.timestamp || Date.now())
      } else if (msg.type === 'pending_messages') {
        const messages = (msg.messages as ServerPersistedMsg[]) ?? []
        for (const pm of messages) {
          if (!pm.groupId) continue // 1:1 messages handled by useIncomingMessages
          decryptAndStore(pm.groupId, pm.id, pm.encryptedPayload, pm.timestamp)
        }
      }
    },
    [decryptAndStore],
  )

  return { handleMessage }
}

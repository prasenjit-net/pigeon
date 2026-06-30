import { useCallback, useEffect, useState } from 'react'
import { encryptMessage, decryptMessage } from '../crypto/encrypt'
import type { WsMessage } from './useWebSocket'
import type { SignedCertificate } from '../crypto/certificate'
import {
  loadConversation,
  appendMessage,
  updateMessageById,
  markRead,
  type PersistedMsg,
} from '../store/messages'

export interface ChatMessage {
  id: string
  clientId?: string
  direction: 'sent' | 'received'
  text: string
  timestamp: number
  status: 'delivered' | 'queued' | 'failed'
  error?: string
}

interface Options {
  recipientId: string
  recipientEncKeyJWK: JsonWebKey
  ownEncPrivKeyJWK: JsonWebKey
  ownCert: SignedCertificate
  send: (payload: object) => void
}

// Module-level map: clientId → { recipientId, text, timestamp }
// Survives re-renders and conversation switches so acks can update the right
// localStorage entry even when the user has navigated to a different chat.
const pendingMap = new Map<string, { recipientId: string; text: string; timestamp: number }>()
let msgSeq = 0

function storedToChat(m: PersistedMsg): ChatMessage {
  return {
    id: m.id,
    clientId: m.clientId,
    direction: m.direction,
    text: m.text,
    timestamp: m.timestamp,
    status: m.status,
    error: m.error,
  }
}

export function useChat({
  recipientId,
  recipientEncKeyJWK,
  ownEncPrivKeyJWK,
  ownCert,
  send,
}: Options) {
  const [messages, setMessages] = useState<ChatMessage[]>(() =>
    recipientId ? loadConversation(recipientId).messages.map(storedToChat) : [],
  )
  const [sending, setSending] = useState(false)

  // Reload from localStorage whenever the selected conversation changes.
  useEffect(() => {
    if (!recipientId) {
      setMessages([])
      return
    }
    setMessages(loadConversation(recipientId).messages.map(storedToChat))
    markRead(recipientId)
  }, [recipientId])

  function appendLocal(msg: ChatMessage) {
    setMessages((prev) => {
      if (prev.some((m) => m.id === msg.id)) return prev
      return [...prev, msg]
    })
  }

  function updateLocal(matchId: string, patch: Partial<ChatMessage>) {
    setMessages((prev) => prev.map((m) => (m.id === matchId ? { ...m, ...patch } : m)))
  }

  const sendMessage = useCallback(
    async (text: string) => {
      if (!text.trim() || !recipientId) return
      setSending(true)
      const clientId = `client-${++msgSeq}`
      const timestamp = Date.now()

      // Track so the ack handler can update localStorage for the right conversation.
      pendingMap.set(clientId, { recipientId, text, timestamp })

      // Optimistic: show immediately as 'queued'.
      const optimistic: ChatMessage = {
        id: clientId,
        clientId,
        direction: 'sent',
        text,
        timestamp,
        status: 'queued',
      }
      appendLocal(optimistic)
      // Persist immediately so the message survives a page reload.
      appendMessage(recipientId, { ...optimistic } as PersistedMsg)

      try {
        const encryptedPayload = await encryptMessage(text, recipientEncKeyJWK)
        send({
          type: 'message',
          clientMsgId: clientId,
          to: recipientId,
          encryptedPayload,
          senderCert: ownCert,
        })
      } catch (err) {
        const errMsg = `Encryption failed: ${err}`
        updateLocal(clientId, { status: 'failed', error: errMsg })
        updateMessageById(recipientId, clientId, { status: 'failed', error: errMsg })
        pendingMap.delete(clientId)
        setSending(false)
        return
      }
      setSending(false)
    },
    [recipientId, recipientEncKeyJWK, ownCert, send],
  )

  const receiveMessage = useCallback(
    async (msg: WsMessage) => {
      if (msg.type === 'message_ack') {
        const clientMsgId = msg.clientMsgId as string
        const serverMsgId = msg.serverMsgId as string
        const status: ChatMessage['status'] =
          (msg.status as string) === 'delivered' ? 'delivered' : 'queued'
        const timestamp = msg.timestamp as number

        // Update state for the currently viewed conversation.
        updateLocal(clientMsgId, { id: serverMsgId, status, timestamp })

        // Always update localStorage via the module-level map (works even if the
        // user has switched to a different conversation).
        const pending = pendingMap.get(clientMsgId)
        if (pending) {
          updateMessageById(pending.recipientId, clientMsgId, {
            id: serverMsgId,
            status,
            timestamp,
          })
          pendingMap.delete(clientMsgId)
        }
        return
      }

      if (msg.type !== 'message') return
      if (!recipientId || (msg.from as string) !== recipientId) return

      const serverMsgId = msg.id as string
      const timestamp = (msg.timestamp as number) || Date.now()
      try {
        const plaintext = await decryptMessage(msg.encryptedPayload as string, ownEncPrivKeyJWK)
        const chatMsg: ChatMessage = {
          id: serverMsgId,
          direction: 'received',
          text: plaintext,
          timestamp,
          status: 'delivered',
        }
        appendLocal(chatMsg)
        appendMessage(recipientId, chatMsg as PersistedMsg)
      } catch (err) {
        const id = serverMsgId || `err-${++msgSeq}`
        appendLocal({
          id,
          direction: 'received',
          text: '',
          timestamp,
          status: 'failed',
          error: `Decryption failed: ${err}`,
        })
      }
    },
    [recipientId, ownEncPrivKeyJWK],
  )

  return { messages, sending, sendMessage, receiveMessage }
}

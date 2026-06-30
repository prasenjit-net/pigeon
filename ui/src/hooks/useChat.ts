import { useCallback, useEffect, useState } from 'react'
import { encryptMessage } from '../crypto/encrypt'
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

// useChat manages the encrypted message history for a single open conversation.
// Decryption and localStorage persistence of incoming messages happens
// upstream in useIncomingMessages, which runs regardless of which
// conversation (if any) is selected; this hook is only responsible for
// sending, ack bookkeeping, and rendering the currently open conversation.
export function useChat({ recipientId, recipientEncKeyJWK, ownCert, send }: Options) {
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

  // Handles message_ack frames from the server after a send.
  const receiveAck = useCallback((msg: WsMessage) => {
    if (msg.type !== 'message_ack') return
    const clientMsgId = msg.clientMsgId as string
    const serverMsgId = msg.serverMsgId as string
    const status: ChatMessage['status'] = (msg.status as string) === 'delivered' ? 'delivered' : 'queued'
    const timestamp = msg.timestamp as number

    // Update state for the currently viewed conversation (no-op if it's not this one).
    updateLocal(clientMsgId, { id: serverMsgId, status, timestamp })

    // Always update localStorage via the module-level map (works even if the
    // user has switched to a different conversation since sending).
    const pending = pendingMap.get(clientMsgId)
    if (pending) {
      updateMessageById(pending.recipientId, clientMsgId, {
        id: serverMsgId,
        status,
        timestamp,
      })
      pendingMap.delete(clientMsgId)
    }
  }, [])

  // Called by the parent when a message for this open conversation has
  // already been decrypted and persisted (see useIncomingMessages).
  const addIncomingMessage = useCallback(
    (msg: ChatMessage) => {
      appendLocal(msg)
      if (recipientId) markRead(recipientId)
    },
    [recipientId],
  )

  return { messages, sending, sendMessage, receiveAck, addIncomingMessage }
}

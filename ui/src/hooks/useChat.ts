import { useCallback, useRef, useState } from 'react'
import { encryptMessage, decryptMessage } from '../crypto/encrypt'
import type { WsMessage } from './useWebSocket'
import type { SignedCertificate } from '../crypto/certificate'

export interface ChatMessage {
  id: string
  direction: 'sent' | 'received'
  text: string
  timestamp: number
  error?: string
}

interface Options {
  recipientId: string
  recipientEncKeyJWK: JsonWebKey
  ownEncPrivKeyJWK: JsonWebKey
  ownCert: SignedCertificate
  send: (payload: object) => void
}

let msgSeq = 0

// useChat manages the encrypted message history for a single conversation.
export function useChat({ recipientId, recipientEncKeyJWK, ownEncPrivKeyJWK, ownCert, send }: Options) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [sending, setSending] = useState(false)
  const pendingRef = useRef<Map<string, string>>(new Map()) // id → plaintext while encrypting

  function append(msg: ChatMessage) {
    setMessages((prev) => [...prev, msg])
  }

  const sendMessage = useCallback(
    async (text: string) => {
      if (!text.trim()) return
      setSending(true)
      const id = `msg-${++msgSeq}`
      try {
        const encryptedPayload = await encryptMessage(text, recipientEncKeyJWK)
        send({
          type: 'message',
          to: recipientId,
          encryptedPayload,
          senderCert: ownCert,
        })
        append({ id, direction: 'sent', text, timestamp: Date.now() })
      } catch (err) {
        append({ id, direction: 'sent', text: '', timestamp: Date.now(), error: `Encryption failed: ${err}` })
      } finally {
        setSending(false)
      }
    },
    [recipientId, recipientEncKeyJWK, ownCert, send],
  )

  const receiveMessage = useCallback(
    async (msg: WsMessage) => {
      if (msg.type !== 'message') return
      if ((msg.from as string) !== recipientId) return

      const id = `msg-${++msgSeq}`
      try {
        const plaintext = await decryptMessage(msg.encryptedPayload as string, ownEncPrivKeyJWK)
        append({ id, direction: 'received', text: plaintext, timestamp: Date.now() })
      } catch (err) {
        append({ id, direction: 'received', text: '', timestamp: Date.now(), error: `Decryption failed: ${err}` })
      }
    },
    [recipientId, ownEncPrivKeyJWK],
  )

  return { messages, sending, sendMessage, receiveMessage }
}

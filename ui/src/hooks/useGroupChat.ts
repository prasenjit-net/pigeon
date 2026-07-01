import { useCallback, useEffect, useState } from 'react'
import { groupEncryptMessage } from '../crypto/encrypt'
import type { WsMessage } from './useWebSocket'
import type { SignedCertificate } from '../crypto/certificate'
import type { KnownGroup } from './useGroups'
import {
  loadGroupConversation,
  appendGroupMessage,
  updateGroupMessageById,
  markGroupRead,
  type PersistedMsg,
} from '../store/messages'
import type { ChatMessage } from './useChat'

interface Options {
  group: KnownGroup | null
  ownId: string
  ownCert: SignedCertificate
  send: (payload: object) => void
}

// Module-level ack map so acks update localStorage even after conversation switch.
const pendingGroupMap = new Map<string, { groupId: string; timestamp: number }>()
let groupMsgSeq = 0

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

export function useGroupChat({ group, ownId, ownCert, send }: Options) {
  const [messages, setMessages] = useState<ChatMessage[]>(() =>
    group ? loadGroupConversation(group.id).messages.map(storedToChat) : [],
  )
  const [sending, setSending] = useState(false)

  useEffect(() => {
    if (!group) {
      setMessages([])
      return
    }
    setMessages(loadGroupConversation(group.id).messages.map(storedToChat))
    markGroupRead(group.id)
  }, [group?.id])

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
      if (!text.trim() || !group) return
      setSending(true)
      const clientId = `gclient-${++groupMsgSeq}`
      const timestamp = Date.now()

      pendingGroupMap.set(clientId, { groupId: group.id, timestamp })

      const optimistic: ChatMessage = {
        id: clientId,
        clientId,
        direction: 'sent',
        text,
        timestamp,
        status: 'queued',
      }
      appendLocal(optimistic)
      appendGroupMessage(group.id, { ...optimistic } as PersistedMsg)

      // Include sender (ownId) in member list so they can verify their own sent copy.
      const encMembers = group.members.some((m) => m.id === ownId)
        ? group.members
        : [...group.members, { id: ownId, encryptionKey: ownCert.cert.subject.encryptionKey } as (typeof group.members)[0]]

      try {
        const encryptedPayload = await groupEncryptMessage(
          text,
          encMembers.map((m) => ({ id: m.id, encryptionKey: m.encryptionKey })),
        )
        send({
          type: 'group_message',
          clientMsgId: clientId,
          groupId: group.id,
          encryptedPayload,
          senderCert: ownCert,
        })
      } catch (err) {
        const errMsg = `Encryption failed: ${err}`
        updateLocal(clientId, { status: 'failed', error: errMsg })
        updateGroupMessageById(group.id, clientId, { status: 'failed', error: errMsg })
        pendingGroupMap.delete(clientId)
        setSending(false)
        return
      }
      setSending(false)
    },
    [group, ownId, ownCert, send],
  )

  const receiveAck = useCallback((msg: WsMessage) => {
    if (msg.type !== 'group_message_ack') return
    const clientMsgId = msg.clientMsgId as string
    const serverMsgId = msg.serverMsgId as string
    const status: ChatMessage['status'] = (msg.status as string) === 'delivered' ? 'delivered' : 'queued'
    const timestamp = msg.timestamp as number

    updateLocal(clientMsgId, { id: serverMsgId, status, timestamp })

    const pending = pendingGroupMap.get(clientMsgId)
    if (pending) {
      updateGroupMessageById(pending.groupId, clientMsgId, { id: serverMsgId, status, timestamp })
      pendingGroupMap.delete(clientMsgId)
    }
  }, [])

  const addIncomingMessage = useCallback(
    (msg: ChatMessage) => {
      appendLocal(msg)
      if (group) markGroupRead(group.id)
    },
    [group?.id],
  )

  return { messages, sending, sendMessage, receiveAck, addIncomingMessage }
}

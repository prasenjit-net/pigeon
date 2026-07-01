import { useCallback, useRef, useState } from 'react'
import type { StoredIdentity } from '../store/identity'
import { markRead, unreadCount } from '../store/messages'
import { useWebSocket, type WsMessage } from '../hooks/useWebSocket'
import { useUsers, type KnownUser } from '../hooks/useUsers'
import { useChat, type ChatMessage } from '../hooks/useChat'
import { useIncomingMessages } from '../hooks/useIncomingMessages'
import UserList from '../components/chat/UserList'
import ConversationPane from '../components/chat/ConversationPane'

interface Props {
  identity: StoredIdentity
  onLogout: (reason?: string) => void
}

export default function ChatPage({ identity, onLogout }: Props) {
  const [selected, setSelected] = useState<KnownUser | null>(null)
  const [unreadCounts, setUnreadCounts] = useState<Map<string, number>>(new Map())
  const ownId = identity.certificate.cert.subject.id

  // sendRef lets useChat call send without creating a circular hook dependency.
  const sendRef = useRef<(payload: object) => void>(() => {})

  const { users, handleMessage: usersHandler } = useUsers(ownId)

  const chat = useChat(
    selected
      ? {
          recipientId: selected.id,
          recipientEncKeyJWK: selected.encryptionKey,
          ownCert: identity.certificate,
          send: (p) => sendRef.current(p),
        }
      : {
          recipientId: '',
          recipientEncKeyJWK: {},
          ownCert: identity.certificate,
          send: () => {},
        },
  )

  const bumpUnread = useCallback((fromId: string) => {
    setUnreadCounts((prev) => {
      const next = new Map(prev)
      next.set(fromId, unreadCount(fromId))
      return next
    })
  }, [])

  // Fires for every incoming message regardless of which conversation (if
  // any) is currently open — decryption and localStorage persistence must
  // not depend on the UI selection, otherwise messages for an unopened
  // conversation are dropped on the floor.
  const handleIncoming = useCallback(
    (fromId: string, msg: ChatMessage) => {
      if (selected?.id === fromId) {
        chat.addIncomingMessage(msg)
      } else {
        bumpUnread(fromId)
      }
    },
    [selected, chat.addIncomingMessage, bumpUnread],
  )

  const { handleMessage: handleIncomingWs } = useIncomingMessages({
    ownEncPrivKeyJWK: identity.encryptionPrivateKey,
    onMessage: handleIncoming,
  })

  const handleMessage = useCallback(
    (msg: WsMessage) => {
      usersHandler(msg)
      handleIncomingWs(msg)
      chat.receiveAck(msg)
    },
    [usersHandler, handleIncomingWs, chat.receiveAck],
  )

  const { send } = useWebSocket({
    certificate: identity.certificate,
    onMessage: handleMessage,
    onFatalError: () => onLogout('invalid_cert'),
  })
  sendRef.current = send

  function handleSelectUser(u: KnownUser) {
    setSelected(u)
    markRead(u.id)
    setUnreadCounts((prev) => {
      const next = new Map(prev)
      next.set(u.id, 0)
      return next
    })
  }

  function handleLogout() {
    onLogout()
  }

  return (
    <div className="flex h-screen bg-white dark:bg-gray-950 overflow-hidden">
      {/* Sidebar */}
      <div className="flex flex-col w-64 min-w-[14rem] border-r border-gray-200 dark:border-gray-800">
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900">
          <span className="text-base font-bold text-gray-900 dark:text-white tracking-tight">🕊️ Pigeon</span>
          <button
            onClick={handleLogout}
            className="text-xs text-gray-400 hover:text-red-500 dark:hover:text-red-400 transition-colors"
          >
            Reset
          </button>
        </div>
        <UserList
          users={users}
          selectedId={selected?.id ?? null}
          unreadCounts={unreadCounts}
          onSelect={handleSelectUser}
          ownName={identity.name}
        />
      </div>

      {/* Main panel */}
      <main className="flex flex-1 flex-col min-w-0">
        {selected ? (
          <ConversationPane
            recipient={selected}
            messages={chat.messages}
            sending={chat.sending}
            onSend={chat.sendMessage}
          />
        ) : (
          <div className="flex flex-1 items-center justify-center">
            <div className="text-center space-y-3 px-4">
              <p className="text-5xl">🕊️</p>
              <p className="text-sm text-gray-500 dark:text-gray-400 max-w-xs">
                Select someone from the sidebar to start an end-to-end encrypted conversation.
              </p>
            </div>
          </div>
        )}
      </main>
    </div>
  )
}

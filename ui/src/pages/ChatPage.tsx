import { useCallback, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import type { StoredIdentity } from '../store/identity'
import { clearIdentity } from '../store/identity'
import { useWebSocket, type WsMessage } from '../hooks/useWebSocket'
import { useRoster, type OnlineUser } from '../hooks/useRoster'
import { useChat } from '../hooks/useChat'
import UserList from '../components/chat/UserList'
import ConversationPane from '../components/chat/ConversationPane'

interface Props {
  identity: StoredIdentity
}

export default function ChatPage({ identity }: Props) {
  const navigate = useNavigate()
  const [selected, setSelected] = useState<OnlineUser | null>(null)
  const ownId = identity.certificate.cert.subject.id

  // sendRef lets useChat call send without creating a circular hook dependency.
  const sendRef = useRef<(payload: object) => void>(() => {})

  const { roster, handleMessage: rosterHandler } = useRoster(ownId)

  const chat = useChat(
    selected
      ? {
          recipientId: selected.id,
          recipientEncKeyJWK: selected.encryptionKey,
          ownEncPrivKeyJWK: identity.encryptionPrivateKey,
          ownCert: identity.certificate,
          send: (p) => sendRef.current(p),
        }
      : {
          // Dummy args when no user is selected — chat state is never rendered.
          recipientId: '',
          recipientEncKeyJWK: {},
          ownEncPrivKeyJWK: identity.encryptionPrivateKey,
          ownCert: identity.certificate,
          send: () => {},
        },
  )

  const handleMessage = useCallback(
    (msg: WsMessage) => {
      rosterHandler(msg)
      chat.receiveMessage(msg)
    },
    [rosterHandler, chat.receiveMessage], // eslint-disable-line react-hooks/exhaustive-deps
  )

  const { send } = useWebSocket({ certificate: identity.certificate, onMessage: handleMessage })
  sendRef.current = send

  function handleLogout() {
    clearIdentity()
    navigate('/', { replace: true })
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
          users={roster}
          selectedId={selected?.id ?? null}
          onSelect={(u) => { setSelected(u) }}
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

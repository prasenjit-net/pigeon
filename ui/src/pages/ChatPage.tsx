import { useCallback, useRef, useState } from 'react'
import type { StoredIdentity } from '../store/identity'
import { markRead, unreadCount, groupUnreadCount, markGroupRead } from '../store/messages'
import { useWebSocket, type WsMessage } from '../hooks/useWebSocket'
import { useUsers, type KnownUser } from '../hooks/useUsers'
import { useChat, type ChatMessage } from '../hooks/useChat'
import { useIncomingMessages } from '../hooks/useIncomingMessages'
import { useConnections } from '../hooks/useConnections'
import { useGroups, type KnownGroup } from '../hooks/useGroups'
import { useGroupChat } from '../hooks/useGroupChat'
import { useGroupIncomingMessages } from '../hooks/useGroupIncomingMessages'
import UserList from '../components/chat/UserList'
import ConversationPane from '../components/chat/ConversationPane'
import AddContactPanel from '../components/chat/AddContactPanel'
import PendingRequestsList from '../components/chat/PendingRequestsList'
import GroupList from '../components/chat/GroupList'
import GroupPendingInvites from '../components/chat/GroupPendingInvites'
import CreateGroupPanel from '../components/chat/CreateGroupPanel'
import GroupConversationPane from '../components/chat/GroupConversationPane'

type ConversationTarget =
  | { kind: 'dm'; user: KnownUser }
  | { kind: 'group'; group: KnownGroup }
  | null

interface Props {
  identity: StoredIdentity
  onLogout: (reason?: string) => void
}

export default function ChatPage({ identity, onLogout }: Props) {
  const [selected, setSelected] = useState<ConversationTarget>(null)
  const [unreadCounts, setUnreadCounts] = useState<Map<string, number>>(new Map())
  const [groupUnreadCounts, setGroupUnreadCounts] = useState<Map<string, number>>(new Map())
  const [invitedMap, setInvitedMap] = useState<Map<string, Set<string>>>(new Map())

  const ownId = identity.certificate.cert.subject.id

  const sendRef = useRef<(payload: object) => void>(() => {})

  const { users, handleMessage: usersHandler } = useUsers(ownId)

  const chat = useChat(
    selected?.kind === 'dm'
      ? {
          recipientId: selected.user.id,
          recipientEncKeyJWK: selected.user.encryptionKey,
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

  const groups = useGroups(ownId, (p) => sendRef.current(p))

  const groupChat = useGroupChat({
    group: selected?.kind === 'group' ? selected.group : null,
    ownId,
    ownCert: identity.certificate,
    send: (p) => sendRef.current(p),
  })

  const bumpUnread = useCallback((fromId: string) => {
    setUnreadCounts((prev) => {
      const next = new Map(prev)
      next.set(fromId, unreadCount(fromId))
      return next
    })
  }, [])

  const bumpGroupUnread = useCallback((groupId: string) => {
    setGroupUnreadCounts((prev) => {
      const next = new Map(prev)
      next.set(groupId, groupUnreadCount(groupId))
      return next
    })
  }, [])

  const handleIncoming = useCallback(
    (fromId: string, msg: ChatMessage) => {
      if (selected?.kind === 'dm' && selected.user.id === fromId) {
        chat.addIncomingMessage(msg)
      } else {
        bumpUnread(fromId)
      }
    },
    [selected, chat.addIncomingMessage, bumpUnread],
  )

  const handleGroupIncoming = useCallback(
    (groupId: string, msg: ChatMessage) => {
      if (selected?.kind === 'group' && selected.group.id === groupId) {
        groupChat.addIncomingMessage(msg)
      } else {
        bumpGroupUnread(groupId)
      }
    },
    [selected, groupChat.addIncomingMessage, bumpGroupUnread],
  )

  const { handleMessage: handleIncomingWs } = useIncomingMessages({
    ownEncPrivKeyJWK: identity.encryptionPrivateKey,
    onMessage: handleIncoming,
  })

  const { handleMessage: handleGroupIncomingWs } = useGroupIncomingMessages({
    ownId,
    ownEncPrivKeyJWK: identity.encryptionPrivateKey,
    onGroupMessage: handleGroupIncoming,
  })

  const connections = useConnections((payload) => sendRef.current(payload))

  const handleMessage = useCallback(
    (msg: WsMessage) => {
      usersHandler(msg)
      handleIncomingWs(msg)
      chat.receiveAck(msg)
      connections.handleMessage(msg)
      groups.handleMessage(msg)
      handleGroupIncomingWs(msg)
      groupChat.receiveAck(msg)
    },
    [
      usersHandler,
      handleIncomingWs,
      chat.receiveAck,
      connections.handleMessage,
      groups.handleMessage,
      handleGroupIncomingWs,
      groupChat.receiveAck,
    ],
  )

  const { send } = useWebSocket({
    certificate: identity.certificate,
    onMessage: handleMessage,
    onFatalError: (code) => onLogout(code),
  })
  sendRef.current = send

  function handleSelectUser(u: KnownUser) {
    setSelected({ kind: 'dm', user: u })
    markRead(u.id)
    setUnreadCounts((prev) => {
      const next = new Map(prev)
      next.set(u.id, 0)
      return next
    })
  }

  function handleSelectGroup(g: KnownGroup) {
    setSelected({ kind: 'group', group: g })
    markGroupRead(g.id)
    setGroupUnreadCounts((prev) => {
      const next = new Map(prev)
      next.set(g.id, 0)
      return next
    })
  }

  function handleInvite(groupId: string, targetId: string) {
    groups.inviteToGroup(groupId, targetId)
    setInvitedMap((prev) => {
      const next = new Map(prev)
      const existing = new Set(next.get(groupId) ?? [])
      existing.add(targetId)
      next.set(groupId, existing)
      return next
    })
  }

  return (
    <div className="flex h-screen bg-white dark:bg-gray-950 overflow-hidden">
      {/* Sidebar */}
      <div className="flex flex-col w-64 min-w-[14rem] border-r border-gray-200 dark:border-gray-800 overflow-y-auto">
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 sticky top-0 z-10">
          <div>
            <span className="text-base font-bold text-gray-900 dark:text-white tracking-tight">🕊️ Pigeon</span>
            <p className="text-xs text-gray-400 dark:text-gray-500">@{identity.handle}</p>
          </div>
          <button
            onClick={() => onLogout()}
            className="text-xs text-gray-400 hover:text-red-500 dark:hover:text-red-400 transition-colors"
          >
            Reset
          </button>
        </div>

        <PendingRequestsList
          requests={connections.pendingInbound}
          onRespond={connections.respond}
        />

        <GroupPendingInvites
          invites={groups.pendingInvites}
          onRespond={groups.respondToInvite}
        />

        <AddContactPanel
          onSendRequest={connections.sendRequest}
          sentTo={connections.sentTo}
        />

        <CreateGroupPanel onCreateGroup={groups.createGroup} />

        <UserList
          users={users}
          selectedId={selected?.kind === 'dm' ? selected.user.id : null}
          unreadCounts={unreadCounts}
          onSelect={handleSelectUser}
          ownName={identity.name}
        />

        <GroupList
          groups={groups.groups}
          selectedId={selected?.kind === 'group' ? selected.group.id : null}
          unreadCounts={groupUnreadCounts}
          onSelect={handleSelectGroup}
        />
      </div>

      {/* Main panel */}
      <main className="flex flex-1 flex-col min-w-0">
        {selected?.kind === 'dm' && (
          <ConversationPane
            recipient={selected.user}
            messages={chat.messages}
            sending={chat.sending}
            onSend={chat.sendMessage}
          />
        )}
        {selected?.kind === 'group' && (
          <GroupConversationPane
            group={selected.group}
            ownId={ownId}
            messages={groupChat.messages}
            sending={groupChat.sending}
            onSend={groupChat.sendMessage}
            onInvite={handleInvite}
            invitedIds={invitedMap.get(selected.group.id) ?? new Set()}
          />
        )}
        {!selected && (
          <div className="flex flex-1 items-center justify-center">
            <div className="text-center space-y-3 px-4">
              <p className="text-5xl">🕊️</p>
              <p className="text-sm text-gray-500 dark:text-gray-400 max-w-xs">
                Add contacts or create a group to start an end-to-end encrypted conversation.
              </p>
            </div>
          </div>
        )}
      </main>
    </div>
  )
}

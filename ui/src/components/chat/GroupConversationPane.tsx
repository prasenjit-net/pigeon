import { useEffect, useRef } from 'react'
import MessageBubble from './MessageBubble'
import MessageInput from './MessageInput'
import GroupInvitePanel from './GroupInvitePanel'
import type { ChatMessage } from '../../hooks/useChat'
import type { KnownGroup } from '../../hooks/useGroups'

interface Props {
  group: KnownGroup
  ownId: string
  messages: ChatMessage[]
  sending: boolean
  onSend: (text: string) => void
  onInvite: (groupId: string, targetId: string) => void
  invitedIds: Set<string>
}

export default function GroupConversationPane({
  group,
  ownId,
  messages,
  sending,
  onSend,
  onInvite,
  invitedIds,
}: Props) {
  const bottomRef = useRef<HTMLDivElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80
    if (atBottom) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
    }
  }, [messages])

  const isOwner = group.ownerId === ownId
  const onlineCount = group.members.filter((m) => m.online).length

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center gap-3 px-4 py-3 border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 min-w-0">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-sm font-semibold text-gray-900 dark:text-white truncate">
              {group.name}
            </span>
            <span className="text-xs text-gray-400 dark:text-gray-500 font-mono flex-shrink-0">
              #{group.handle}
            </span>
          </div>
          <p className="text-xs text-gray-400 dark:text-gray-500">
            {group.members.length} member{group.members.length !== 1 ? 's' : ''} · {onlineCount} online
          </p>
        </div>
        {isOwner && (
          <div className="flex-shrink-0 max-w-xs">
            <GroupInvitePanel
              groupId={group.id}
              onInvite={onInvite}
              sentTo={invitedIds}
            />
          </div>
        )}
      </div>

      {/* Messages */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto px-4 py-4 space-y-3">
        {messages.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full text-center py-16">
            <div className="text-4xl mb-3">🔒</div>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Messages are end-to-end encrypted.
              <br />
              Only members of <strong>#{group.handle}</strong> can read them.
            </p>
          </div>
        )}
        {messages.map((msg) => (
          <MessageBubble key={msg.id} message={msg} />
        ))}
        <div ref={bottomRef} />
      </div>

      <MessageInput onSend={onSend} disabled={sending} />
    </div>
  )
}

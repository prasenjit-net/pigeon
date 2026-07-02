import { useEffect, useRef } from 'react'
import clsx from 'clsx'
import MessageBubble from './MessageBubble'
import MessageInput from './MessageInput'
import type { ChatMessage } from '../../hooks/useChat'
import type { KnownUser } from '../../hooks/useUsers'

interface Props {
  recipient: KnownUser
  messages: ChatMessage[]
  sending: boolean
  onSend: (text: string) => void
  onDisconnect: () => void
}

export default function ConversationPane({ recipient, messages, sending, onSend, onDisconnect }: Props) {
  const bottomRef = useRef<HTMLDivElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom when new messages arrive, unless user has scrolled up.
  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80
    if (atBottom) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
    }
  }, [messages])

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center gap-2 px-4 py-3 border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
        <span
          className={clsx(
            'flex h-2 w-2 rounded-full',
            recipient.online ? 'bg-green-400' : 'bg-gray-300 dark:bg-gray-600',
          )}
        />
        <span className="text-sm font-semibold text-gray-900 dark:text-white">{recipient.name}</span>
        <span className="text-xs text-gray-400 dark:text-gray-600">
          {recipient.online ? 'online' : 'offline'}
        </span>
        <span className="ml-1 rounded-full bg-gray-100 dark:bg-gray-800 px-2 py-0.5 text-xs text-gray-500 dark:text-gray-400 font-mono">
          {recipient.id.slice(0, 8)}…
        </span>
        <button
          onClick={onDisconnect}
          title="Disconnect"
          className="ml-auto text-xs text-gray-400 hover:text-red-500 dark:hover:text-red-400 transition-colors px-2 py-1 rounded hover:bg-red-50 dark:hover:bg-red-950/30"
        >
          Disconnect
        </button>
      </div>

      {/* Messages */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto px-4 py-4 space-y-3">
        {messages.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full text-center py-16">
            <div className="text-4xl mb-3">🔒</div>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Messages are end-to-end encrypted.
              <br />
              Only you and <strong>{recipient.name}</strong> can read them.
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

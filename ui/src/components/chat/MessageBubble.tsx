import clsx from 'clsx'
import type { ChatMessage } from '../../hooks/useChat'

interface Props {
  message: ChatMessage
}

export default function MessageBubble({ message }: Props) {
  const sent = message.direction === 'sent'
  const time = new Date(message.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

  return (
    <div className={clsx('flex', sent ? 'justify-end' : 'justify-start')}>
      <div
        className={clsx(
          'max-w-[70%] rounded-2xl px-4 py-2 text-sm shadow-sm',
          sent
            ? 'bg-indigo-600 text-white rounded-br-sm'
            : 'bg-white dark:bg-gray-800 text-gray-900 dark:text-white border border-gray-200 dark:border-gray-700 rounded-bl-sm',
          message.error && 'opacity-60',
        )}
      >
        {message.error ? (
          <span className="italic text-xs opacity-80">{message.error}</span>
        ) : (
          <span className="whitespace-pre-wrap break-words">{message.text}</span>
        )}
        <span
          className={clsx(
            'block text-right text-xs mt-1',
            sent ? 'text-indigo-200' : 'text-gray-400 dark:text-gray-500',
          )}
        >
          {time}
        </span>
      </div>
    </div>
  )
}

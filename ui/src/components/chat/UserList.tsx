import clsx from 'clsx'
import type { OnlineUser } from '../../hooks/useRoster'

interface Props {
  users: OnlineUser[]
  selectedId: string | null
  unreadCounts: Map<string, number>
  onSelect: (user: OnlineUser) => void
  ownName: string
}

export default function UserList({ users, selectedId, unreadCounts, onSelect, ownName }: Props) {
  return (
    <aside className="flex flex-col w-64 min-w-[14rem] border-r border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 h-full">
      <div className="px-4 py-4 border-b border-gray-200 dark:border-gray-800">
        <div className="flex items-center gap-2">
          <span className="flex h-2 w-2 rounded-full bg-green-500" />
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300 truncate">{ownName}</span>
        </div>
        <p className="mt-0.5 text-xs text-gray-400 dark:text-gray-600">You</p>
      </div>

      <div className="px-3 py-3">
        <p className="px-1 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-600 mb-2">
          Online — {users.length}
        </p>
        {users.length === 0 ? (
          <p className="px-1 text-xs text-gray-400 dark:text-gray-600 italic">No other users online</p>
        ) : (
          <ul className="space-y-0.5">
            {users.map((u) => {
              const count = unreadCounts.get(u.id) ?? 0
              return (
                <li key={u.id}>
                  <button
                    onClick={() => onSelect(u)}
                    className={clsx(
                      'flex items-center gap-2 w-full rounded-lg px-2 py-2 text-sm text-left transition-colors',
                      selectedId === u.id
                        ? 'bg-indigo-100 dark:bg-indigo-900/40 text-indigo-700 dark:text-indigo-300'
                        : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800',
                    )}
                  >
                    <span className="flex h-2 w-2 flex-shrink-0 rounded-full bg-green-400" />
                    <span className="truncate flex-1">{u.name}</span>
                    {count > 0 && (
                      <span className="flex-shrink-0 min-w-[1.25rem] h-5 px-1 rounded-full bg-indigo-500 text-white text-xs font-bold flex items-center justify-center leading-none">
                        {count > 99 ? '99+' : count}
                      </span>
                    )}
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </aside>
  )
}

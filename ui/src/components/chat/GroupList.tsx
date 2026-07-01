import clsx from 'clsx'
import type { KnownGroup } from '../../hooks/useGroups'

interface Props {
  groups: KnownGroup[]
  selectedId: string | null
  unreadCounts: Map<string, number>
  onSelect: (group: KnownGroup) => void
}

export default function GroupList({ groups, selectedId, unreadCounts, onSelect }: Props) {
  if (groups.length === 0) return null

  return (
    <div className="px-3 pt-3 pb-1">
      <p className="px-1 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-600 mb-2">
        Groups — {groups.length}
      </p>
      <ul className="space-y-0.5">
        {groups.map((g) => {
          const unread = unreadCounts.get(g.id) ?? 0
          const selected = selectedId === g.id
          return (
            <li key={g.id}>
              <button
                onClick={() => onSelect(g)}
                className={clsx(
                  'flex items-center gap-2 w-full rounded-lg px-2 py-2 text-sm text-left transition-colors',
                  selected
                    ? 'bg-indigo-100 dark:bg-indigo-900/40 text-indigo-700 dark:text-indigo-300'
                    : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800',
                )}
              >
                <span className="flex-shrink-0 w-5 h-5 flex items-center justify-center rounded bg-gray-200 dark:bg-gray-700 text-gray-500 dark:text-gray-400 text-xs font-bold">
                  #
                </span>
                <span className="truncate flex-1">
                  {g.name}
                  <span className="ml-1 text-xs text-gray-400 dark:text-gray-500">#{g.handle}</span>
                </span>
                {unread > 0 && (
                  <span className="flex-shrink-0 min-w-[1.25rem] h-5 px-1 rounded-full bg-indigo-500 text-white text-xs font-bold flex items-center justify-center leading-none">
                    {unread > 99 ? '99+' : unread}
                  </span>
                )}
              </button>
            </li>
          )
        })}
      </ul>
    </div>
  )
}

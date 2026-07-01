import type { PendingGroupInvite } from '../../hooks/useGroups'

interface Props {
  invites: PendingGroupInvite[]
  onRespond: (memberId: string, accept: boolean) => void
}

export default function GroupPendingInvites({ invites, onRespond }: Props) {
  if (invites.length === 0) return null

  return (
    <div className="border-b border-gray-100 dark:border-gray-800">
      <div className="px-4 py-2 flex items-center gap-1.5">
        <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">
          Group invites
        </span>
        <span className="inline-flex items-center justify-center h-4 min-w-4 px-1 rounded-full bg-indigo-600 text-white text-[10px] font-bold">
          {invites.length > 99 ? '99+' : invites.length}
        </span>
      </div>
      <ul className="pb-2 space-y-1">
        {invites.map((inv) => (
          <li key={inv.memberId} className="px-4 py-1.5">
            <div className="text-sm font-medium text-gray-800 dark:text-gray-200">
              #{inv.group.handle}
              <span className="ml-1 text-xs text-gray-500 dark:text-gray-400 font-normal">
                {inv.group.name}
              </span>
            </div>
            <div className="text-xs text-gray-400 dark:text-gray-500 mb-1">
              from @{inv.inviter.handle}
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => onRespond(inv.memberId, true)}
                className="text-xs px-2 py-0.5 rounded bg-indigo-600 text-white hover:bg-indigo-700"
              >
                Join
              </button>
              <button
                onClick={() => onRespond(inv.memberId, false)}
                className="text-xs px-2 py-0.5 rounded border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
              >
                Decline
              </button>
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}

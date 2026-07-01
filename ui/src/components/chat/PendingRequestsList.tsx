import type { PendingRequest } from '../../hooks/useConnections'

interface Props {
  requests: PendingRequest[]
  onRespond: (connectionId: string, accept: boolean) => void
}

export default function PendingRequestsList({ requests, onRespond }: Props) {
  if (requests.length === 0) return null

  return (
    <div className="border-b border-gray-100 dark:border-gray-800">
      <div className="px-4 py-2 flex items-center gap-1.5">
        <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">
          Requests
        </span>
        <span className="inline-flex items-center justify-center h-4 min-w-4 px-1 rounded-full bg-indigo-600 text-white text-[10px] font-bold">
          {requests.length > 99 ? '99+' : requests.length}
        </span>
      </div>
      <ul className="pb-2 space-y-1">
        {requests.map((req) => (
          <li key={req.connectionId} className="px-4 py-1.5">
            <div className="text-sm font-medium text-gray-800 dark:text-gray-200">
              {req.requester.name}
              <span className="ml-1 text-xs text-gray-500 dark:text-gray-400 font-normal">
                @{req.requester.handle}
              </span>
            </div>
            <div className="mt-1 flex gap-2">
              <button
                onClick={() => onRespond(req.connectionId, true)}
                className="text-xs px-2 py-0.5 rounded bg-indigo-600 text-white hover:bg-indigo-700"
              >
                Accept
              </button>
              <button
                onClick={() => onRespond(req.connectionId, false)}
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

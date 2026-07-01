import { useState } from 'react'
import { usersApi, type UserSearchResult } from '../../services/api'

interface Props {
  groupId: string
  onInvite: (groupId: string, targetId: string) => void
  sentTo: Set<string>
}

export default function GroupInvitePanel({ groupId, onInvite, sentTo }: Props) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [result, setResult] = useState<UserSearchResult | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [searching, setSearching] = useState(false)

  async function handleSearch(e: React.FormEvent) {
    e.preventDefault()
    const handle = query.trim().replace(/^@/, '')
    if (!handle) return
    setSearching(true)
    setResult(null)
    setNotFound(false)
    try {
      const user = await usersApi.search(handle)
      setResult(user)
    } catch {
      setNotFound(true)
    } finally {
      setSearching(false)
    }
  }

  function handleInvite() {
    if (!result) return
    onInvite(groupId, result.id)
    setResult(null)
    setQuery('')
    setOpen(false)
  }

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="text-xs px-2 py-1 rounded border border-indigo-300 dark:border-indigo-700 text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-900/30"
      >
        + Invite
      </button>
    )
  }

  return (
    <div className="flex flex-col gap-2 min-w-0 flex-1">
      <form onSubmit={handleSearch} className="flex gap-2">
        <div className="flex flex-1 rounded-md border border-gray-300 dark:border-gray-600 overflow-hidden text-sm min-w-0">
          <span className="flex items-center px-2 bg-gray-50 dark:bg-gray-700 text-gray-400">@</span>
          <input
            autoFocus
            type="text"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setResult(null)
              setNotFound(false)
            }}
            placeholder="handle"
            className="flex-1 px-2 py-1 bg-white dark:bg-gray-800 text-gray-900 dark:text-white outline-none text-sm min-w-0"
          />
        </div>
        <button
          type="submit"
          disabled={searching || !query.trim()}
          className="px-2 py-1 text-xs rounded-md bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-40 flex-shrink-0"
        >
          {searching ? '…' : 'Find'}
        </button>
        <button
          type="button"
          onClick={() => { setOpen(false); setQuery(''); setResult(null); setNotFound(false) }}
          className="px-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 flex-shrink-0"
        >
          ✕
        </button>
      </form>

      {result && (
        <div className="flex items-center justify-between rounded-md bg-gray-50 dark:bg-gray-800 px-3 py-1.5">
          <div className="text-sm">
            <span className="font-medium text-gray-900 dark:text-white">{result.name}</span>
            <span className="ml-1 text-gray-500 dark:text-gray-400 text-xs">@{result.handle}</span>
          </div>
          {sentTo.has(result.id) ? (
            <span className="text-xs text-gray-400">Invited</span>
          ) : (
            <button
              onClick={handleInvite}
              className="text-xs px-2 py-0.5 rounded bg-indigo-600 text-white hover:bg-indigo-700"
            >
              Invite
            </button>
          )}
        </div>
      )}

      {notFound && (
        <p className="text-xs text-gray-500 dark:text-gray-400">No user found with that handle.</p>
      )}
    </div>
  )
}

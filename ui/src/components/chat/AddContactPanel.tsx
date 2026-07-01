import { useState } from 'react'
import { usersApi, type UserSearchResult } from '../../services/api'

interface Props {
  onSendRequest: (targetId: string) => void
  sentTo: Set<string>
}

export default function AddContactPanel({ onSendRequest, sentTo }: Props) {
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

  function handleAdd() {
    if (!result) return
    onSendRequest(result.id)
    setResult(null)
    setQuery('')
  }

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="flex items-center gap-1 px-4 py-2 text-xs text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-200"
      >
        <span className="text-base leading-none">+</span> Add contact
      </button>
    )
  }

  return (
    <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-800">
      <form onSubmit={handleSearch} className="flex gap-2">
        <div className="flex flex-1 rounded-md border border-gray-300 dark:border-gray-600 overflow-hidden text-sm">
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
            className="flex-1 px-2 py-1.5 bg-white dark:bg-gray-800 text-gray-900 dark:text-white outline-none"
          />
        </div>
        <button
          type="submit"
          disabled={searching || !query.trim()}
          className="px-3 py-1.5 text-xs rounded-md bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-40"
        >
          {searching ? '…' : 'Find'}
        </button>
        <button
          type="button"
          onClick={() => { setOpen(false); setQuery(''); setResult(null); setNotFound(false) }}
          className="px-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
        >
          ✕
        </button>
      </form>

      {result && (
        <div className="mt-2 flex items-center justify-between rounded-md bg-gray-50 dark:bg-gray-800 px-3 py-2">
          <div className="text-sm">
            <span className="font-medium text-gray-900 dark:text-white">{result.name}</span>
            <span className="ml-1 text-gray-500 dark:text-gray-400 text-xs">@{result.handle}</span>
          </div>
          {sentTo.has(result.id) ? (
            <span className="text-xs text-gray-400">Pending…</span>
          ) : (
            <button
              onClick={handleAdd}
              className="text-xs px-2 py-1 rounded bg-indigo-600 text-white hover:bg-indigo-700"
            >
              Add
            </button>
          )}
        </div>
      )}

      {notFound && (
        <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">No user found with that handle.</p>
      )}
    </div>
  )
}

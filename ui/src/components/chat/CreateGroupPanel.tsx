import { useState } from 'react'

const GROUP_HANDLE_RE = /^[a-z][a-z0-9_]{2,31}$/

interface Props {
  onCreateGroup: (handle: string, name: string) => void
}

export default function CreateGroupPanel({ onCreateGroup }: Props) {
  const [open, setOpen] = useState(false)
  const [handle, setHandle] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState('')

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const h = handle.trim().replace(/^#/, '')
    const n = name.trim()
    if (!GROUP_HANDLE_RE.test(h)) {
      setError('Handle must be 3–32 chars: lowercase letters, digits, underscores.')
      return
    }
    if (!n) {
      setError('Group name is required.')
      return
    }
    onCreateGroup(h, n)
    setOpen(false)
    setHandle('')
    setName('')
    setError('')
  }

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="flex items-center gap-1 px-4 py-2 text-xs text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-200"
      >
        <span className="text-base leading-none">+</span> New group
      </button>
    )
  }

  return (
    <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-800">
      <form onSubmit={handleSubmit} className="space-y-2">
        <div className="flex rounded-md border border-gray-300 dark:border-gray-600 overflow-hidden text-sm">
          <span className="flex items-center px-2 bg-gray-50 dark:bg-gray-700 text-gray-400">#</span>
          <input
            autoFocus
            type="text"
            value={handle}
            onChange={(e) => {
              setHandle(e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, ''))
              setError('')
            }}
            placeholder="handle"
            maxLength={32}
            className="flex-1 px-2 py-1.5 bg-white dark:bg-gray-800 text-gray-900 dark:text-white outline-none text-sm"
          />
        </div>
        <input
          type="text"
          value={name}
          onChange={(e) => { setName(e.target.value); setError('') }}
          placeholder="Group name"
          maxLength={64}
          className="w-full px-2 py-1.5 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white outline-none text-sm"
        />
        {error && <p className="text-xs text-red-500">{error}</p>}
        <div className="flex gap-2">
          <button
            type="submit"
            disabled={!handle || !name}
            className="flex-1 py-1.5 text-xs rounded-md bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-40"
          >
            Create
          </button>
          <button
            type="button"
            onClick={() => { setOpen(false); setHandle(''); setName(''); setError('') }}
            className="px-3 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-xs"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  )
}

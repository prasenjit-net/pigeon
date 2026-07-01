import { useState } from 'react'

export interface NameStepPayload {
  name: string
  handle: string
}

interface Props {
  onNext: (payload: NameStepPayload) => void
}

const HANDLE_RE = /^[a-z][a-z0-9_]{2,31}$/

export default function NameStep({ onNext }: Props) {
  const [name, setName] = useState('')
  const [handle, setHandle] = useState('')

  const nameError = name.length > 64 ? 'Name must be 64 characters or fewer.' : ''
  const handleError = handle && !HANDLE_RE.test(handle)
    ? 'Handle must be 3–32 characters: lowercase letters, digits, underscores; must start with a letter.'
    : ''
  const canSubmit = name.trim() && !nameError && handle && !handleError

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!canSubmit) return
    onNext({ name: name.trim(), handle })
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-gray-900 dark:text-white">Welcome to Pigeon</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Choose a display name and a unique handle so others can find you.
        </p>
      </div>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label htmlFor="name" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
            Display name
          </label>
          <input
            id="name"
            type="text"
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. Alice"
            className="mt-1 block w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-gray-900 dark:text-white placeholder-gray-400 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
          {nameError && <p className="mt-1 text-xs text-red-500">{nameError}</p>}
        </div>
        <div>
          <label htmlFor="handle" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
            Handle
          </label>
          <div className="mt-1 flex rounded-md shadow-sm">
            <span className="inline-flex items-center rounded-l-md border border-r-0 border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-700 px-3 text-gray-500 dark:text-gray-400 text-sm">
              @
            </span>
            <input
              id="handle"
              type="text"
              value={handle}
              onChange={(e) => setHandle(e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, ''))}
              placeholder="alice"
              maxLength={32}
              className="block w-full rounded-none rounded-r-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-gray-900 dark:text-white placeholder-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
          </div>
          {handleError && <p className="mt-1 text-xs text-red-500">{handleError}</p>}
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
            Unique, permanent. Others use this to find and add you.
          </p>
        </div>
        <button
          type="submit"
          disabled={!canSubmit}
          className="w-full rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-40 disabled:cursor-not-allowed"
        >
          Continue
        </button>
      </form>
    </div>
  )
}

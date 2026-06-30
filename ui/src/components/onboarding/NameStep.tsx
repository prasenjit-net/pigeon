import { useState } from 'react'

interface Props {
  onNext: (name: string) => void
}

export default function NameStep({ onNext }: Props) {
  const [name, setName] = useState('')
  const error = name.length > 64 ? 'Name must be 64 characters or fewer.' : ''

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = name.trim()
    if (!trimmed || trimmed.length > 64) return
    onNext(trimmed)
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-gray-900 dark:text-white">Welcome to Pigeon</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Choose a display name. This will appear alongside your encrypted identity.
        </p>
      </div>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label htmlFor="name" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
            Your name
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
          {error && <p className="mt-1 text-xs text-red-500">{error}</p>}
        </div>
        <button
          type="submit"
          disabled={!name.trim() || !!error}
          className="w-full rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-40 disabled:cursor-not-allowed"
        >
          Continue
        </button>
      </form>
    </div>
  )
}

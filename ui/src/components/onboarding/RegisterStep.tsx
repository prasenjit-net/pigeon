import { useEffect, useState } from 'react'
import type { GeneratedKeys } from './KeygenStep'
import { saveIdentity, saveCaPublicKey } from '../../store/identity'

interface Props {
  keys: GeneratedKeys
  onDone: () => void
  onError: (msg: string) => void
}

export default function RegisterStep({ keys, onDone, onError }: Props) {
  const [status, setStatus] = useState<'registering' | 'done' | 'error'>('registering')

  useEffect(() => {
    let cancelled = false

    async function run() {
      try {
        // Fetch and cache the CA public key for future certificate verification.
        const caRes = await fetch('/api/ca/public-key')
        if (!caRes.ok) throw new Error('Failed to fetch CA public key')
        const caJWK: JsonWebKey = await caRes.json()
        saveCaPublicKey(caJWK)

        // Register with the server.
        const res = await fetch('/api/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: keys.name,
            id: keys.id,
            signingKey: keys.signingPublicKey,
            encryptionKey: keys.encryptionPublicKey,
          }),
        })
        if (!res.ok) {
          const body = await res.json().catch(() => ({}))
          throw new Error(body.error ?? `HTTP ${res.status}`)
        }

        const certificate = await res.json()

        saveIdentity({
          name: keys.name,
          signingPublicKey: keys.signingPublicKey,
          signingPrivateKey: keys.signingPrivateKey,
          encryptionPublicKey: keys.encryptionPublicKey,
          encryptionPrivateKey: keys.encryptionPrivateKey,
          certificate,
        })

        if (!cancelled) {
          setStatus('done')
          onDone()
        }
      } catch (err) {
        if (!cancelled) {
          setStatus('error')
          onError(String(err))
        }
      }
    }

    run()
    return () => { cancelled = true }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-gray-900 dark:text-white">Registering with server</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          The server is signing your certificate. This binds your name to your public keys.
        </p>
      </div>
      <div className="flex items-center gap-3 text-sm">
        {status === 'registering' && (
          <>
            <svg className="animate-spin h-5 w-5 text-indigo-600" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
            </svg>
            <span className="text-gray-600 dark:text-gray-400">Signing certificate…</span>
          </>
        )}
        {status === 'done' && (
          <>
            <span className="flex h-5 w-5 items-center justify-center rounded-full bg-green-500 text-white text-xs">✓</span>
            <span className="text-green-600 dark:text-green-400 font-medium">Certificate issued! Entering chat…</span>
          </>
        )}
        {status === 'error' && (
          <span className="text-red-500">Registration failed — see error above.</span>
        )}
      </div>
    </div>
  )
}

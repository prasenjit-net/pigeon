import { useEffect, useState } from 'react'
import {
  generateSigningKeyPair,
  generateEncryptionKeyPair,
  exportPublicKeyJWK,
  exportPrivateKeyJWK,
} from '../../crypto/keys'
import { fingerprintJWK } from '../../crypto/fingerprint'

export interface GeneratedKeys {
  name: string
  handle: string
  id: string
  signingPublicKey: JsonWebKey
  signingPrivateKey: JsonWebKey
  encryptionPublicKey: JsonWebKey
  encryptionPrivateKey: JsonWebKey
}

interface Props {
  name: string
  handle: string
  onDone: (keys: GeneratedKeys) => void
  onError: (msg: string) => void
}

type Step = 'signing' | 'encryption' | 'fingerprint' | 'done'

export default function KeygenStep({ name, handle, onDone, onError }: Props) {
  const [step, setStep] = useState<Step>('signing')

  useEffect(() => {
    let cancelled = false

    async function run() {
      try {
        setStep('signing')
        const sigPair = await generateSigningKeyPair()

        setStep('encryption')
        const encPair = await generateEncryptionKeyPair()

        setStep('fingerprint')
        const sigPub = await exportPublicKeyJWK(sigPair.publicKey)
        const sigPriv = await exportPrivateKeyJWK(sigPair.privateKey)
        const encPub = await exportPublicKeyJWK(encPair.publicKey)
        const encPriv = await exportPrivateKeyJWK(encPair.privateKey)
        const id = await fingerprintJWK(sigPub)

        setStep('done')
        if (!cancelled) {
          onDone({ name, handle, id, signingPublicKey: sigPub, signingPrivateKey: sigPriv, encryptionPublicKey: encPub, encryptionPrivateKey: encPriv })
        }
      } catch (err) {
        if (!cancelled) onError(String(err))
      }
    }

    run()
    return () => { cancelled = true }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const steps: { key: Step; label: string }[] = [
    { key: 'signing', label: 'Generating signing key pair (RSA-PSS 2048)…' },
    { key: 'encryption', label: 'Generating encryption key pair (RSA-OAEP 2048)…' },
    { key: 'fingerprint', label: 'Computing identity fingerprint…' },
    { key: 'done', label: 'Keys ready' },
  ]

  const currentIdx = steps.findIndex((s) => s.key === step)

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-gray-900 dark:text-white">Generating your keys</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Your private keys never leave this browser. Please wait a moment.
        </p>
      </div>
      <ul className="space-y-3">
        {steps.map((s, idx) => {
          const done = idx < currentIdx || step === 'done'
          const active = idx === currentIdx && step !== 'done'
          return (
            <li key={s.key} className="flex items-center gap-3 text-sm">
              {done ? (
                <span className="flex h-5 w-5 items-center justify-center rounded-full bg-green-500 text-white text-xs">✓</span>
              ) : active ? (
                <span className="flex h-5 w-5 items-center justify-center">
                  <svg className="animate-spin h-4 w-4 text-indigo-600" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                  </svg>
                </span>
              ) : (
                <span className="flex h-5 w-5 items-center justify-center rounded-full border border-gray-300 dark:border-gray-600" />
              )}
              <span className={active ? 'text-indigo-600 dark:text-indigo-400 font-medium' : done ? 'text-gray-600 dark:text-gray-400' : 'text-gray-400 dark:text-gray-600'}>
                {s.label}
              </span>
            </li>
          )
        })}
      </ul>
    </div>
  )
}

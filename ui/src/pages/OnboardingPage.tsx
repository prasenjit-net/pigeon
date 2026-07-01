import { useState } from 'react'
import NameStep, { type NameStepPayload } from '../components/onboarding/NameStep'
import KeygenStep, { type GeneratedKeys } from '../components/onboarding/KeygenStep'
import RegisterStep from '../components/onboarding/RegisterStep'

const REASON_MESSAGES: Record<string, string> = {
  invalid_cert: 'Your identity is no longer accepted by the server — the server may have been reset. Please register again.',
  cert_outdated: 'Your identity certificate is outdated. Please re-register to get a new one.',
}

type Step = 'name' | 'keygen' | 'register'

interface Props {
  onRegistered: () => void
  reason?: string | null
}

export default function OnboardingPage({ onRegistered, reason }: Props) {
  const [step, setStep] = useState<Step>('name')
  const [identity, setIdentity] = useState<NameStepPayload | null>(null)
  const [keys, setKeys] = useState<GeneratedKeys | null>(null)
  const [error, setError] = useState<string | null>(null)

  const steps: Step[] = ['name', 'keygen', 'register']

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-950 flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-white dark:bg-gray-900 rounded-2xl shadow-lg p-8">
        {reason && REASON_MESSAGES[reason] && (
          <div className="mb-6 rounded-md bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 px-4 py-3 text-sm text-amber-800 dark:text-amber-300">
            {REASON_MESSAGES[reason]}
          </div>
        )}

        {/* Step indicator */}
        <div className="flex items-center gap-2 mb-8">
          {steps.map((s, i) => (
            <div key={s} className="flex items-center gap-2">
              <div
                className={`h-2 w-2 rounded-full transition-colors ${
                  s === step
                    ? 'bg-indigo-600'
                    : steps.indexOf(s) < steps.indexOf(step)
                      ? 'bg-green-500'
                      : 'bg-gray-200 dark:bg-gray-700'
                }`}
              />
              {i < steps.length - 1 && <div className="h-px w-6 bg-gray-200 dark:bg-gray-700" />}
            </div>
          ))}
        </div>

        {error && (
          <div className="mb-6 rounded-md bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 px-4 py-3 text-sm text-red-700 dark:text-red-400">
            {error}
            <button
              className="ml-3 underline text-xs"
              onClick={() => { setError(null); setStep('name') }}
            >
              Start over
            </button>
          </div>
        )}

        {step === 'name' && (
          <NameStep
            onNext={(payload) => {
              setIdentity(payload)
              setStep('keygen')
            }}
          />
        )}
        {step === 'keygen' && identity && (
          <KeygenStep
            name={identity.name}
            handle={identity.handle}
            onDone={(k) => { setKeys(k); setStep('register') }}
            onError={setError}
          />
        )}
        {step === 'register' && keys && (
          <RegisterStep keys={keys} onDone={onRegistered} onError={setError} />
        )}
      </div>
    </div>
  )
}

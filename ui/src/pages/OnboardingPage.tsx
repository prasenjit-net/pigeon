import { useState } from 'react'
import NameStep from '../components/onboarding/NameStep'
import KeygenStep, { type GeneratedKeys } from '../components/onboarding/KeygenStep'
import RegisterStep from '../components/onboarding/RegisterStep'

type Step = 'name' | 'keygen' | 'register'

interface Props {
  onRegistered: () => void
}

export default function OnboardingPage({ onRegistered }: Props) {
  const [step, setStep] = useState<Step>('name')
  const [name, setName] = useState('')
  const [keys, setKeys] = useState<GeneratedKeys | null>(null)
  const [error, setError] = useState<string | null>(null)

  const steps: Step[] = ['name', 'keygen', 'register']

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-950 flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-white dark:bg-gray-900 rounded-2xl shadow-lg p-8">
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
          <NameStep onNext={(n) => { setName(n); setStep('keygen') }} />
        )}
        {step === 'keygen' && (
          <KeygenStep
            name={name}
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

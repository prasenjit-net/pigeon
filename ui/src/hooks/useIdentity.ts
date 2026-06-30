import { useEffect, useState } from 'react'
import { loadIdentity, type StoredIdentity } from '../store/identity'

export type IdentityState =
  | { status: 'loading' }
  | { status: 'missing' }
  | { status: 'ready'; identity: StoredIdentity }

export function useIdentity(): IdentityState {
  const [state, setState] = useState<IdentityState>({ status: 'loading' })

  useEffect(() => {
    const identity = loadIdentity()
    setState(identity ? { status: 'ready', identity } : { status: 'missing' })
  }, [])

  return state
}

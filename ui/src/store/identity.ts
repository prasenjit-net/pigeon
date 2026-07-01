// Persists the user's cryptographic identity in localStorage.
// Private keys are stored as JWK — the Web Crypto API does not provide a
// more secure browser storage mechanism for extractable keys.

import type { SignedCertificate } from '../crypto/certificate'

const KEY = {
  signingPrivate: 'pigeon.identity.signingPrivateKey',
  signingPublic: 'pigeon.identity.signingPublicKey',
  encPrivate: 'pigeon.identity.encryptionPrivateKey',
  encPublic: 'pigeon.identity.encryptionPublicKey',
  certificate: 'pigeon.identity.certificate',
  name: 'pigeon.identity.name',
  handle: 'pigeon.identity.handle',
  caPublicKey: 'pigeon.ca.publicKey',
} as const

export interface StoredIdentity {
  name: string
  handle: string
  signingPrivateKey: JsonWebKey
  signingPublicKey: JsonWebKey
  encryptionPrivateKey: JsonWebKey
  encryptionPublicKey: JsonWebKey
  certificate: SignedCertificate
}

export function saveIdentity(identity: StoredIdentity): void {
  localStorage.setItem(KEY.name, identity.name)
  localStorage.setItem(KEY.handle, identity.handle)
  localStorage.setItem(KEY.signingPrivate, JSON.stringify(identity.signingPrivateKey))
  localStorage.setItem(KEY.signingPublic, JSON.stringify(identity.signingPublicKey))
  localStorage.setItem(KEY.encPrivate, JSON.stringify(identity.encryptionPrivateKey))
  localStorage.setItem(KEY.encPublic, JSON.stringify(identity.encryptionPublicKey))
  localStorage.setItem(KEY.certificate, JSON.stringify(identity.certificate))
}

export function loadIdentity(): StoredIdentity | null {
  try {
    const name = localStorage.getItem(KEY.name)
    const handle = localStorage.getItem(KEY.handle)
    const sigPub = localStorage.getItem(KEY.signingPublic)
    const sigPriv = localStorage.getItem(KEY.signingPrivate)
    const encPub = localStorage.getItem(KEY.encPublic)
    const encPriv = localStorage.getItem(KEY.encPrivate)
    const cert = localStorage.getItem(KEY.certificate)

    if (!name || !handle || !sigPub || !sigPriv || !encPub || !encPriv || !cert) return null

    return {
      name,
      handle,
      signingPublicKey: JSON.parse(sigPub),
      signingPrivateKey: JSON.parse(sigPriv),
      encryptionPublicKey: JSON.parse(encPub),
      encryptionPrivateKey: JSON.parse(encPriv),
      certificate: JSON.parse(cert),
    }
  } catch {
    return null
  }
}

export function clearIdentity(): void {
  Object.values(KEY).forEach((k) => localStorage.removeItem(k))
}

export function saveCaPublicKey(jwk: JsonWebKey): void {
  localStorage.setItem(KEY.caPublicKey, JSON.stringify(jwk))
}

export function loadCaPublicKey(): JsonWebKey | null {
  const raw = localStorage.getItem(KEY.caPublicKey)
  if (!raw) return null
  try {
    return JSON.parse(raw)
  } catch {
    return null
  }
}

// End-to-end message encryption / decryption.
//
// Short messages (≤190 bytes) use RSA-OAEP directly.
// Longer messages use hybrid encryption: a random AES-256-GCM key is
// RSA-OAEP-encrypted; the message body is AES-GCM-encrypted with that key.

import { importEncryptionPublicKey, importEncryptionPrivateKey } from './keys'

const RSA_OAEP_MAX_PLAINTEXT = 190

export interface EncryptedPayload {
  v: 1
  mode: 'rsa-oaep' | 'hybrid'
  encryptedKey?: string
  iv?: string
  ciphertext: string
}

export async function encryptMessage(plaintext: string, recipientEncKeyJWK: JsonWebKey): Promise<string> {
  const pubKey = await importEncryptionPublicKey(recipientEncKeyJWK)
  const data = new TextEncoder().encode(plaintext)

  let payload: EncryptedPayload
  if (data.length <= RSA_OAEP_MAX_PLAINTEXT) {
    const cipherBuf = await crypto.subtle.encrypt({ name: 'RSA-OAEP' }, pubKey, data)
    payload = { v: 1, mode: 'rsa-oaep', ciphertext: toB64(cipherBuf) }
  } else {
    payload = await hybridEncrypt(data, pubKey)
  }
  return JSON.stringify(payload)
}

export async function decryptMessage(payloadStr: string, ownEncPrivKeyJWK: JsonWebKey): Promise<string> {
  const payload: EncryptedPayload = JSON.parse(payloadStr)
  const privKey = await importEncryptionPrivateKey(ownEncPrivKeyJWK)

  let plainBuf: ArrayBuffer
  if (payload.mode === 'rsa-oaep') {
    plainBuf = await crypto.subtle.decrypt({ name: 'RSA-OAEP' }, privKey, fromB64(payload.ciphertext))
  } else if (payload.mode === 'hybrid') {
    plainBuf = await hybridDecrypt(payload, privKey)
  } else {
    throw new Error('unsupported encryption mode')
  }
  return new TextDecoder().decode(plainBuf)
}

async function hybridEncrypt(data: Uint8Array<ArrayBuffer>, rsaPubKey: CryptoKey): Promise<EncryptedPayload> {
  const aesKey = await crypto.subtle.generateKey({ name: 'AES-GCM', length: 256 }, true, ['encrypt'])
  const iv = crypto.getRandomValues(new Uint8Array(12) as Uint8Array<ArrayBuffer>)

  const cipherBuf = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, aesKey, data)
  const rawAesKey = await crypto.subtle.exportKey('raw', aesKey)
  const encAesKey = await crypto.subtle.encrypt({ name: 'RSA-OAEP' }, rsaPubKey, rawAesKey)

  return {
    v: 1,
    mode: 'hybrid',
    encryptedKey: toB64(encAesKey),
    iv: toB64(iv),
    ciphertext: toB64(cipherBuf),
  }
}

async function hybridDecrypt(payload: EncryptedPayload, rsaPrivKey: CryptoKey): Promise<ArrayBuffer> {
  const rawAesKey = await crypto.subtle.decrypt({ name: 'RSA-OAEP' }, rsaPrivKey, fromB64(payload.encryptedKey!))
  const aesKey = await crypto.subtle.importKey('raw', rawAesKey, { name: 'AES-GCM' }, false, ['decrypt'])
  return crypto.subtle.decrypt({ name: 'AES-GCM', iv: fromB64(payload.iv!) }, aesKey, fromB64(payload.ciphertext))
}

// ── Group encryption ─────────────────────────────────────────────────────────
//
// Per-message multi-recipient key wrapping: each group message generates a
// fresh AES-256-GCM key, encrypts the plaintext once, then wraps the AES key
// with every member's RSA-OAEP public key. The server routes one ciphertext
// blob to all members; each member unwraps their own copy of the AES key.

export interface GroupEncryptedPayload {
  v: 1
  ciphertext: string
  iv: string
  wrappedKeys: { [userId: string]: string }
}

export async function groupEncryptMessage(
  plaintext: string,
  members: Array<{ id: string; encryptionKey: JsonWebKey }>,
): Promise<string> {
  const aesKey = await crypto.subtle.generateKey({ name: 'AES-GCM', length: 256 }, true, ['encrypt'])
  const iv = crypto.getRandomValues(new Uint8Array(12) as Uint8Array<ArrayBuffer>)
  const data = new TextEncoder().encode(plaintext)
  const cipherBuf = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, aesKey, data)
  const rawAesKey = await crypto.subtle.exportKey('raw', aesKey)

  const wrappedKeys: GroupEncryptedPayload['wrappedKeys'] = {}
  for (const m of members) {
    const pub = await importEncryptionPublicKey(m.encryptionKey)
    const wrapped = await crypto.subtle.encrypt({ name: 'RSA-OAEP' }, pub, rawAesKey)
    wrappedKeys[m.id] = toB64(wrapped)
  }

  const payload: GroupEncryptedPayload = {
    v: 1,
    ciphertext: toB64(cipherBuf),
    iv: toB64(iv),
    wrappedKeys,
  }
  return JSON.stringify(payload)
}

export async function groupDecryptMessage(
  payloadStr: string,
  ownId: string,
  ownEncPrivKeyJWK: JsonWebKey,
): Promise<string> {
  const payload: GroupEncryptedPayload = JSON.parse(payloadStr)
  const wrapped = payload.wrappedKeys[ownId]
  if (!wrapped) throw new Error('groupDecryptMessage: no wrapped key for own id')
  const priv = await importEncryptionPrivateKey(ownEncPrivKeyJWK)
  const rawAes = await crypto.subtle.decrypt({ name: 'RSA-OAEP' }, priv, fromB64(wrapped))
  const aesKey = await crypto.subtle.importKey('raw', rawAes, { name: 'AES-GCM' }, false, ['decrypt'])
  const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv: fromB64(payload.iv) }, aesKey, fromB64(payload.ciphertext))
  return new TextDecoder().decode(plain)
}

// base64url encode from any buffer-like source
function toB64(buf: ArrayBuffer | Uint8Array<ArrayBuffer>): string {
  const bytes = buf instanceof ArrayBuffer ? new Uint8Array(buf) : buf
  return btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '')
}

// base64url decode to a concrete ArrayBuffer-backed Uint8Array
function fromB64(s: string): Uint8Array<ArrayBuffer> {
  const b64 = s.replace(/-/g, '+').replace(/_/g, '/').padEnd(s.length + ((4 - (s.length % 4)) % 4), '=')
  const binary = atob(b64)
  const bytes = new Uint8Array(binary.length) as Uint8Array<ArrayBuffer>
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

// Certificate types and browser-side verification.
// The CA public key is fetched from /api/ca/public-key and cached in
// localStorage — it is used to verify every SignedCertificate received.

export interface CertSubject {
  name: string
  id: string
  signingKey: JsonWebKey
  encryptionKey: JsonWebKey
}

export interface Cert {
  version: number
  subject: CertSubject
  issuedAt: number
  expiresAt: number
  issuer: string
}

export interface SignedCertificate {
  cert: Cert
  signature: string // base64url RSA-PSS signature over canonical JSON of cert
}

// verifyCertificate returns true if the certificate signature is valid and
// the certificate has not expired.
export async function verifyCertificate(
  signed: SignedCertificate,
  caPublicKeyJWK: JsonWebKey,
): Promise<boolean> {
  try {
    const now = Math.floor(Date.now() / 1000)
    if (now > signed.cert.expiresAt) return false

    const caKey = await crypto.subtle.importKey(
      'jwk',
      caPublicKeyJWK,
      { name: 'RSA-PSS', hash: 'SHA-256' },
      false,
      ['verify'],
    )

    const canonical = new TextEncoder().encode(JSON.stringify(signed.cert))
    const sigBytes = fromBase64URL(signed.signature)

    return crypto.subtle.verify({ name: 'RSA-PSS', saltLength: 32 }, caKey, sigBytes, canonical)
  } catch {
    return false
  }
}

function fromBase64URL(s: string): Uint8Array<ArrayBuffer> {
  const b64 = s.replace(/-/g, '+').replace(/_/g, '/').padEnd(s.length + ((4 - (s.length % 4)) % 4), '=')
  const binary = atob(b64)
  const bytes = new Uint8Array(binary.length) as Uint8Array<ArrayBuffer>
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

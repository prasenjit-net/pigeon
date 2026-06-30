// Compute a stable hex fingerprint of a JWK public key.
// Only the key material fields (kty, n, e for RSA) are hashed — not
// algorithm-specific metadata — so the fingerprint is the same regardless
// of which algorithm the JWK was exported with.

export async function fingerprintJWK(jwk: JsonWebKey): Promise<string> {
  // Canonical subset: only the fields that identify the key material.
  const canonical: Record<string, unknown> = { kty: jwk.kty, n: jwk.n, e: jwk.e }
  const json = JSON.stringify(canonical, Object.keys(canonical).sort())
  const encoded = new TextEncoder().encode(json)
  const hashBuf = await crypto.subtle.digest('SHA-256', encoded)
  return Array.from(new Uint8Array(hashBuf))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

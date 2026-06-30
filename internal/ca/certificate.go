package ca

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// CertSubject is the user identity included in a certificate.
type CertSubject struct {
	Name          string         `json:"name"`
	ID            string         `json:"id"` // hex SHA-256 of signing key JWK
	SigningKey     map[string]any `json:"signingKey"`
	EncryptionKey map[string]any `json:"encryptionKey"`
}

// Cert is the unsigned certificate payload.
type Cert struct {
	Version   int         `json:"version"`
	Subject   CertSubject `json:"subject"`
	IssuedAt  int64       `json:"issuedAt"`
	ExpiresAt int64       `json:"expiresAt"`
	Issuer    string      `json:"issuer"`
}

// SignedCertificate is what the server returns to the client and what clients
// present when connecting via WebSocket.
type SignedCertificate struct {
	Cert      Cert   `json:"cert"`
	Signature string `json:"signature"` // base64url RSA-PSS signature of canonical cert JSON
}

const certVersion = 1
const certValidityDays = 365
const issuerName = "pigeon"

// Issue builds and signs a certificate for a user.
func (ca *CA) Issue(name, userID string, signingKey, encryptionKey map[string]any) (SignedCertificate, error) {
	now := time.Now().UTC()
	cert := Cert{
		Version: certVersion,
		Subject: CertSubject{
			Name:          name,
			ID:            userID,
			SigningKey:     signingKey,
			EncryptionKey: encryptionKey,
		},
		IssuedAt:  now.Unix(),
		ExpiresAt: now.AddDate(0, 0, certValidityDays).Unix(),
		Issuer:    issuerName,
	}

	canonical, err := canonicalJSON(cert)
	if err != nil {
		return SignedCertificate{}, fmt.Errorf("ca: marshal cert: %w", err)
	}

	sig, err := ca.Sign(canonical)
	if err != nil {
		return SignedCertificate{}, err
	}

	return SignedCertificate{
		Cert:      cert,
		Signature: base64.RawURLEncoding.EncodeToString(sig),
	}, nil
}

// VerifyCertificate checks that a SignedCertificate was issued by this CA and
// has not expired.
func (ca *CA) VerifyCertificate(sc SignedCertificate) error {
	if sc.Cert.Version != certVersion {
		return fmt.Errorf("ca: unsupported certificate version %d", sc.Cert.Version)
	}
	if sc.Cert.Issuer != issuerName {
		return fmt.Errorf("ca: unknown issuer %q", sc.Cert.Issuer)
	}
	if time.Now().Unix() > sc.Cert.ExpiresAt {
		return fmt.Errorf("ca: certificate expired")
	}

	canonical, err := canonicalJSON(sc.Cert)
	if err != nil {
		return fmt.Errorf("ca: marshal cert for verify: %w", err)
	}

	sig, err := base64.RawURLEncoding.DecodeString(sc.Signature)
	if err != nil {
		return fmt.Errorf("ca: decode signature: %w", err)
	}

	if err := ca.Verify(canonical, sig); err != nil {
		return fmt.Errorf("ca: invalid signature: %w", err)
	}
	return nil
}

// canonicalJSON produces a deterministic JSON encoding (sorted keys via
// standard library marshaling of a struct, which is field-order stable).
func canonicalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

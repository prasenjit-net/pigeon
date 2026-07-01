package ca

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// CA is the server Certificate Authority. It holds an RSA-PSS key pair used
// to sign user certificates.
type CA struct {
	mu         sync.RWMutex
	privateKey *rsa.PrivateKey
	logger     *slog.Logger
}

// NewWithStore initialises the CA using the provided KeyStore. If the store
// returns ErrKeyNotFound a new RSA-2048 key pair is generated and saved;
// otherwise the existing PEM is parsed and loaded.
func NewWithStore(store KeyStore, logger *slog.Logger) (*CA, error) {
	ca := &CA{logger: logger}

	pemBytes, err := store.Load()
	switch {
	case errors.Is(err, ErrKeyNotFound):
		logger.Info("ca: generating new CA key pair")
		if err := ca.generateAndSave(store); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, fmt.Errorf("ca: load key: %w", err)
	default:
		if err := ca.parsePEM(pemBytes); err != nil {
			return nil, err
		}
		logger.Info("ca: loaded existing key pair")
	}

	return ca, nil
}

// New is a backward-compatible constructor for tests and the old file-based
// startup path. Production code in server.go uses NewWithStore directly.
func New(dataDir string, logger *slog.Logger) (*CA, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("ca: create data dir: %w", err)
	}
	return NewWithStore(fileKeyStore{rootDir: dataDir}, logger)
}

func (ca *CA) generateAndSave(store KeyStore) error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("ca: generate key: %w", err)
	}

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}
	pemBytes := pem.EncodeToMemory(block)
	if err := store.Save(pemBytes); err != nil {
		return err
	}

	ca.privateKey = priv
	return nil
}

func (ca *CA) parsePEM(data []byte) error {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return fmt.Errorf("ca: invalid PEM block")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("ca: parse key: %w", err)
	}
	ca.privateKey = priv
	return nil
}

// PublicKeyJWK returns the CA public key as a JSON Web Key map so the browser
// can import it with SubtleCrypto.importKey.
func (ca *CA) PublicKeyJWK() (map[string]any, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	pub := &ca.privateKey.PublicKey
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("ca: marshal public key: %w", err)
	}

	jwk := rsaPublicKeyToJWK(pub, pubDER)
	return jwk, nil
}

// Sign signs data with the CA private key using RSA-PSS SHA-256.
func (ca *CA) Sign(data []byte) ([]byte, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	hash := crypto.SHA256
	h := hash.New()
	h.Write(data)
	digest := h.Sum(nil)

	sig, err := rsa.SignPSS(rand.Reader, ca.privateKey, hash, digest, nil)
	if err != nil {
		return nil, fmt.Errorf("ca: sign: %w", err)
	}
	return sig, nil
}

// Verify checks an RSA-PSS SHA-256 signature against the CA public key.
func (ca *CA) Verify(data, sig []byte) error {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	hash := crypto.SHA256
	h := hash.New()
	h.Write(data)
	digest := h.Sum(nil)

	return rsa.VerifyPSS(&ca.privateKey.PublicKey, hash, digest, sig, nil)
}

// rsaPublicKeyToJWK builds the JWK representation understood by SubtleCrypto.
func rsaPublicKeyToJWK(pub *rsa.PublicKey, _ []byte) map[string]any {
	_ = json.Marshal // keep import
	return map[string]any{
		"kty": "RSA",
		"alg": "PS256",
		"use": "sig",
		"n":   base64URLEncode(pub.N.Bytes()),
		"e":   base64URLEncodeUint(uint64(pub.E)),
	}
}

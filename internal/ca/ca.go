package ca

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// CA is the server Certificate Authority. It holds an RSA-PSS key pair used
// to sign user certificates. The key pair is generated once and persisted to
// disk so it survives restarts.
type CA struct {
	mu         sync.RWMutex
	privateKey *rsa.PrivateKey
	dataDir    string
	logger     *slog.Logger
}

const keyFile = "ca.key.pem"

// New loads the CA key pair from dataDir/ca.key.pem, or generates and saves a
// new one if the file does not exist.
func New(dataDir string, logger *slog.Logger) (*CA, error) {
	ca := &CA{dataDir: dataDir, logger: logger}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("ca: create data dir: %w", err)
	}

	path := filepath.Join(dataDir, keyFile)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		logger.Info("ca: generating new CA key pair")
		if err := ca.generate(path); err != nil {
			return nil, err
		}
	} else {
		if err := ca.load(path); err != nil {
			return nil, err
		}
	}

	logger.Info("ca: ready", "key_file", path)
	return ca, nil
}

func (ca *CA) generate(path string) error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("ca: generate key: %w", err)
	}

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("ca: open key file: %w", err)
	}
	defer f.Close()
	if err := pem.Encode(f, block); err != nil {
		return fmt.Errorf("ca: write key file: %w", err)
	}

	ca.privateKey = priv
	return nil
}

func (ca *CA) load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("ca: read key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return fmt.Errorf("ca: invalid PEM block in %s", path)
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("ca: parse key: %w", err)
	}

	ca.privateKey = priv
	ca.logger.Info("ca: loaded existing key pair")
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

	// Build a minimal RSA JWK for the browser.
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

package ca

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrKeyNotFound is returned by KeyStore.Load when no key has been persisted yet.
var ErrKeyNotFound = errors.New("ca: key not found")

// KeyStore abstracts persistence of the CA private key PEM.
// Implementations include file-based (for backward compat / tests) and
// database-backed (production).
type KeyStore interface {
	Load() (pemBytes []byte, err error)
	Save(pemBytes []byte) error
}

// fileKeyStore persists the CA key as a PEM file — used by tests and by the
// New() compatibility wrapper.
type fileKeyStore struct {
	rootDir string
}

func (s fileKeyStore) Load() ([]byte, error) {
	path := filepath.Join(s.rootDir, "ca.key.pem")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ca: read key file: %w", err)
	}
	return data, nil
}

func (s fileKeyStore) Save(pemBytes []byte) error {
	path := filepath.Join(s.rootDir, "ca.key.pem")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("ca: open key file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(pemBytes); err != nil {
		return fmt.Errorf("ca: write key file: %w", err)
	}
	return nil
}

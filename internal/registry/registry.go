package registry

import (
	"fmt"
	"sync"

	"github.com/prasenjit-net/pigeon/internal/ca"
)

// MemoryRegistry is a thread-safe in-memory implementation of Registry.
// It is used in tests and as a compile-time check that the interface is satisfied.
type MemoryRegistry struct {
	mu      sync.RWMutex
	users   map[string]ca.SignedCertificate
	handles map[string]string // handle → id
}

// New returns an empty MemoryRegistry.
func New() *MemoryRegistry {
	return &MemoryRegistry{
		users:   make(map[string]ca.SignedCertificate),
		handles: make(map[string]string),
	}
}

// Register stores a verified certificate, overwriting any existing entry for
// the same subject ID.
func (r *MemoryRegistry) Register(cert ca.SignedCertificate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[cert.Cert.Subject.ID] = cert
	r.handles[cert.Cert.Subject.Handle] = cert.Cert.Subject.ID
	return nil
}

// Get returns the certificate for the given user ID.
func (r *MemoryRegistry) Get(id string) (ca.SignedCertificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.users[id]
	if !ok {
		return ca.SignedCertificate{}, fmt.Errorf("registry: user %q not found", id)
	}
	return c, nil
}

// GetByHandle returns the certificate for the given handle.
func (r *MemoryRegistry) GetByHandle(handle string) (ca.SignedCertificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.handles[handle]
	if !ok {
		return ca.SignedCertificate{}, fmt.Errorf("registry: handle %q not found", handle)
	}
	return r.users[id], nil
}

// All returns a copy of all registered certificates.
func (r *MemoryRegistry) All() ([]ca.SignedCertificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ca.SignedCertificate, 0, len(r.users))
	for _, c := range r.users {
		out = append(out, c)
	}
	return out, nil
}

// Compile-time assertion that *MemoryRegistry satisfies Registry.
var _ Registry = (*MemoryRegistry)(nil)

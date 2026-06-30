package registry

import (
	"fmt"
	"sync"

	"github.com/prasenjit-net/pigeon/internal/ca"
)

// Registry keeps a thread-safe map of registered users indexed by their
// subject ID (hex SHA-256 of their signing key).
type Registry struct {
	mu    sync.RWMutex
	users map[string]ca.SignedCertificate
}

func New() *Registry {
	return &Registry{users: make(map[string]ca.SignedCertificate)}
}

// Register stores a verified certificate. The caller is responsible for
// verifying the certificate before calling this.
func (r *Registry) Register(cert ca.SignedCertificate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[cert.Cert.Subject.ID] = cert
}

// Get returns the certificate for a given user ID.
func (r *Registry) Get(id string) (ca.SignedCertificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.users[id]
	if !ok {
		return ca.SignedCertificate{}, fmt.Errorf("registry: user %q not found", id)
	}
	return c, nil
}

// All returns a copy of all registered certificates.
func (r *Registry) All() []ca.SignedCertificate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ca.SignedCertificate, 0, len(r.users))
	for _, c := range r.users {
		out = append(out, c)
	}
	return out
}

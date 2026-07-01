package registry

import "github.com/prasenjit-net/pigeon/internal/ca"

// Registry stores and retrieves registered user certificates. Both the
// in-memory implementation (MemoryRegistry) and the database-backed
// implementation (in internal/db) satisfy this interface.
type Registry interface {
	Register(cert ca.SignedCertificate) error
	Get(id string) (ca.SignedCertificate, error)
	GetByHandle(handle string) (ca.SignedCertificate, error)
	All() ([]ca.SignedCertificate, error)
}

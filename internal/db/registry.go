package db

import (
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/prasenjit-net/pigeon/internal/ca"
	"github.com/prasenjit-net/pigeon/internal/registry"
)

type gormRegistry struct {
	db *gorm.DB
}

// NewGORMRegistry returns a registry.Registry backed by the given *gorm.DB.
func NewGORMRegistry(db *gorm.DB) registry.Registry {
	return &gormRegistry{db: db}
}

func (r *gormRegistry) Register(cert ca.SignedCertificate) error {
	data, err := json.Marshal(cert)
	if err != nil {
		return fmt.Errorf("registry: marshal cert: %w", err)
	}
	row := User{
		ID:          cert.Cert.Subject.ID,
		Handle:      cert.Cert.Subject.Handle,
		Certificate: string(data),
	}
	// Save upserts on primary key conflict.
	return r.db.Save(&row).Error
}

func (r *gormRegistry) GetByHandle(handle string) (ca.SignedCertificate, error) {
	var row User
	if err := r.db.Where("handle = ?", handle).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ca.SignedCertificate{}, fmt.Errorf("registry: handle %q not found", handle)
		}
		return ca.SignedCertificate{}, err
	}
	var cert ca.SignedCertificate
	if err := json.Unmarshal([]byte(row.Certificate), &cert); err != nil {
		return ca.SignedCertificate{}, fmt.Errorf("registry: unmarshal cert: %w", err)
	}
	return cert, nil
}

func (r *gormRegistry) Get(id string) (ca.SignedCertificate, error) {
	var row User
	if err := r.db.First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ca.SignedCertificate{}, fmt.Errorf("registry: user %q not found", id)
		}
		return ca.SignedCertificate{}, err
	}
	var cert ca.SignedCertificate
	if err := json.Unmarshal([]byte(row.Certificate), &cert); err != nil {
		return ca.SignedCertificate{}, fmt.Errorf("registry: unmarshal cert: %w", err)
	}
	return cert, nil
}

func (r *gormRegistry) All() ([]ca.SignedCertificate, error) {
	var rows []User
	if err := r.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	certs := make([]ca.SignedCertificate, 0, len(rows))
	for _, row := range rows {
		var cert ca.SignedCertificate
		if err := json.Unmarshal([]byte(row.Certificate), &cert); err != nil {
			return nil, fmt.Errorf("registry: unmarshal cert for %q: %w", row.ID, err)
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

// Compile-time assertion.
var _ registry.Registry = (*gormRegistry)(nil)

package db

import (
	"errors"

	"gorm.io/gorm"

	"github.com/prasenjit-net/pigeon/internal/ca"
)

type gormKeyStore struct {
	db *gorm.DB
}

// NewGORMKeyStore returns a ca.KeyStore backed by the given *gorm.DB.
func NewGORMKeyStore(db *gorm.DB) ca.KeyStore {
	return &gormKeyStore{db: db}
}

func (s *gormKeyStore) Load() ([]byte, error) {
	var row CAKey
	if err := s.db.First(&row, 1).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ca.ErrKeyNotFound
		}
		return nil, err
	}
	return []byte(row.PEM), nil
}

func (s *gormKeyStore) Save(pemBytes []byte) error {
	row := CAKey{ID: 1, PEM: string(pemBytes)}
	return s.db.Save(&row).Error
}

// Compile-time assertion.
var _ ca.KeyStore = (*gormKeyStore)(nil)

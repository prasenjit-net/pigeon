package db

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// CAKey is the GORM model for the single CA private-key row.
// ID is always 1 so Save() performs an upsert on the primary key.
type CAKey struct {
	ID        uint      `gorm:"primaryKey;autoIncrement:false"`
	PEM       string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// User is the GORM model for a registered user.
// ID is the hex SHA-256 of the user's signing-key JWK (assigned by the client).
type User struct {
	ID          string    `gorm:"primaryKey"`
	Handle      string    `gorm:"uniqueIndex;not null"` // unique @handle (stored without @)
	Certificate string    `gorm:"type:text;not null"`   // JSON-serialised ca.SignedCertificate
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// Connection is the GORM model for a connection request between two users.
type Connection struct {
	ID                string    `gorm:"primaryKey"`
	RequesterID       string    `gorm:"not null;uniqueIndex:idx_conn_pair"`
	RecipientID       string    `gorm:"not null;uniqueIndex:idx_conn_pair"`
	Status            string    `gorm:"not null;default:'pending'"` // pending|accepted|rejected
	RecipientNotified bool      `gorm:"not null;default:false"`     // request delivered to recipient
	RequesterNotified bool      `gorm:"not null;default:false"`     // response delivered to requester
	CreatedAt         time.Time `gorm:"autoCreateTime"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime"`
}

// Group is the GORM model for a chat group.
// Handle is stored without the # prefix; displayed with # in the UI.
type Group struct {
	ID        string    `gorm:"primaryKey"`
	Handle    string    `gorm:"uniqueIndex;not null"` // unique #handle (without #)
	Name      string    `gorm:"not null"`
	OwnerID   string    `gorm:"not null;index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// GroupMember is the GORM model for a user's membership in a group.
// Status: "invited" | "accepted" | "rejected"
type GroupMember struct {
	ID              string    `gorm:"primaryKey"`
	GroupID         string    `gorm:"not null;uniqueIndex:idx_group_member"`
	UserID          string    `gorm:"not null;uniqueIndex:idx_group_member"`
	InviterID       string    `gorm:"not null"`
	Status          string    `gorm:"not null;default:'invited'"`
	InviteeNotified bool      `gorm:"not null;default:false"` // invite delivered to UserID
	InviterNotified bool      `gorm:"not null;default:false"` // response delivered to InviterID
	CreatedAt       time.Time `gorm:"autoCreateTime"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

var gormConfig = &gorm.Config{
	Logger: logger.Default.LogMode(logger.Silent),
}

// Open connects to a PostgreSQL database using dsn and runs AutoMigrate.
func Open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("db: connect postgres: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

// OpenSQLite opens (or creates) a SQLite database at path and runs AutoMigrate.
func OpenSQLite(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("db: open sqlite at %s: %w", path, err)
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

// migrate runs GORM AutoMigrate for all models. It adds missing tables and
// columns but never drops existing ones — safe to run on every startup.
func migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&CAKey{}, &User{}, &Connection{}, &Group{}, &GroupMember{}); err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}
	return nil
}

// Close closes the underlying database connection pool.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

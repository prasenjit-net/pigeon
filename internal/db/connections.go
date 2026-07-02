package db

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/prasenjit-net/pigeon/internal/connections"
	"github.com/prasenjit-net/pigeon/internal/queue"
)

type gormConnectionStore struct {
	db *gorm.DB
}

// NewGORMConnectionStore returns a connections.Store backed by the given *gorm.DB.
func NewGORMConnectionStore(db *gorm.DB) connections.Store {
	return &gormConnectionStore{db: db}
}

func (s *gormConnectionStore) Request(requesterID, recipientID string) (connections.Request, error) {
	row := Connection{
		ID:          queue.NewID(),
		RequesterID: requesterID,
		RecipientID: recipientID,
		Status:      connections.Pending,
	}
	if err := s.db.Create(&row).Error; err != nil {
		if isUniqueConstraintError(err) {
			return connections.Request{}, connections.ErrAlreadyExists
		}
		return connections.Request{}, fmt.Errorf("connections: request: %w", err)
	}
	return toRequest(row), nil
}

func (s *gormConnectionStore) GetByID(id string) (connections.Request, error) {
	var row Connection
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return connections.Request{}, connections.ErrNotFound
		}
		return connections.Request{}, fmt.Errorf("connections: get: %w", err)
	}
	return toRequest(row), nil
}

func (s *gormConnectionStore) UpdateStatus(id, status string) error {
	res := s.db.Model(&Connection{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return fmt.Errorf("connections: update status: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return connections.ErrNotFound
	}
	return nil
}

func (s *gormConnectionStore) MarkRecipientNotified(id string) error {
	return s.db.Model(&Connection{}).Where("id = ?", id).Update("recipient_notified", true).Error
}

func (s *gormConnectionStore) MarkRequesterNotified(id string) error {
	return s.db.Model(&Connection{}).Where("id = ?", id).Update("requester_notified", true).Error
}

func (s *gormConnectionStore) ListAccepted(userID string) ([]connections.Request, error) {
	var rows []Connection
	err := s.db.Where(
		"(requester_id = ? OR recipient_id = ?) AND status = ?",
		userID, userID, connections.Accepted,
	).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("connections: list accepted: %w", err)
	}
	return toRequests(rows), nil
}

func (s *gormConnectionStore) ListUndeliveredInbound(userID string) ([]connections.Request, error) {
	var rows []Connection
	err := s.db.Where(
		"recipient_id = ? AND status = ? AND recipient_notified = ?",
		userID, connections.Pending, false,
	).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("connections: list undelivered inbound: %w", err)
	}
	return toRequests(rows), nil
}

func (s *gormConnectionStore) ListUndeliveredResponses(userID string) ([]connections.Request, error) {
	var rows []Connection
	err := s.db.Where(
		"requester_id = ? AND status != ? AND requester_notified = ?",
		userID, connections.Pending, false,
	).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("connections: list undelivered responses: %w", err)
	}
	return toRequests(rows), nil
}

func (s *gormConnectionStore) Delete(id string) error {
	res := s.db.Delete(&Connection{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("connections: delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return connections.ErrNotFound
	}
	return nil
}

// Compile-time assertion.
var _ connections.Store = (*gormConnectionStore)(nil)

func toRequest(row Connection) connections.Request {
	return connections.Request{
		ID:          row.ID,
		RequesterID: row.RequesterID,
		RecipientID: row.RecipientID,
		Status:      row.Status,
	}
}

func toRequests(rows []Connection) []connections.Request {
	out := make([]connections.Request, len(rows))
	for i, r := range rows {
		out[i] = toRequest(r)
	}
	return out
}

// isUniqueConstraintError detects duplicate key errors across SQLite and Postgres.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// SQLite: "UNIQUE constraint failed"
	// Postgres: "duplicate key value violates unique constraint"
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "duplicate key value")
}

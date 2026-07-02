package connections

import "errors"

type Status = string

const (
	Pending  Status = "pending"
	Accepted Status = "accepted"
	Rejected Status = "rejected"
)

// Request is a connection request between two users.
type Request struct {
	ID          string
	RequesterID string
	RecipientID string
	Status      Status
}

// Store persists connection state. All methods must be goroutine-safe.
type Store interface {
	// Request creates a pending connection from requester → recipient.
	Request(requesterID, recipientID string) (Request, error)

	// GetByID returns the connection with the given ID.
	GetByID(id string) (Request, error)

	// UpdateStatus transitions a connection to accepted or rejected.
	UpdateStatus(id, status string) error

	// MarkRecipientNotified flags that the pending request was delivered to the recipient.
	MarkRecipientNotified(id string) error

	// MarkRequesterNotified flags that the response was delivered to the requester.
	MarkRequesterNotified(id string) error

	// ListAccepted returns all accepted connections for a user (as requester or recipient).
	ListAccepted(userID string) ([]Request, error)

	// ListUndeliveredInbound returns pending requests addressed to userID that
	// have not yet been delivered (RecipientNotified=false).
	ListUndeliveredInbound(userID string) ([]Request, error)

	// ListUndeliveredResponses returns accepted/rejected connections initiated by
	// userID where the response has not yet been delivered (RequesterNotified=false).
	ListUndeliveredResponses(userID string) ([]Request, error)

	// Delete permanently removes a connection record by ID.
	Delete(id string) error
}

var ErrAlreadyExists = errors.New("connections: already exists")
var ErrNotFound = errors.New("connections: not found")

package groups

import "errors"

type MemberStatus = string

const (
	Invited  MemberStatus = "invited"
	Accepted MemberStatus = "accepted"
	Rejected MemberStatus = "rejected"
)

// Group is the domain object for a chat group.
type Group struct {
	ID      string
	Handle  string
	Name    string
	OwnerID string
}

// Member is a group membership record.
type Member struct {
	ID        string
	GroupID   string
	UserID    string
	InviterID string
	Status    MemberStatus
}

// Store persists group and membership state. All methods must be goroutine-safe.
type Store interface {
	// Create creates a new group and auto-inserts the owner as an accepted member.
	// Returns ErrHandleExists if the handle is already taken.
	Create(ownerID, handle, name string) (Group, error)

	// GetByID returns a group by its ID.
	GetByID(id string) (Group, error)

	// GetByHandle returns a group by its handle (without #).
	GetByHandle(handle string) (Group, error)

	// AddMember adds a user as an invited member.
	// Returns ErrAlreadyMember if the (groupID, userID) pair already exists.
	AddMember(groupID, userID, inviterID string) (Member, error)

	// GetMemberByID looks up a membership record by its primary key.
	GetMemberByID(id string) (Member, error)

	// GetMember looks up the membership for a specific user in a specific group.
	GetMember(groupID, userID string) (Member, error)

	// UpdateMemberStatus transitions a membership to accepted or rejected.
	UpdateMemberStatus(memberID, status string) error

	// MarkInviteeNotified flags that the invite was delivered to the invitee.
	MarkInviteeNotified(memberID string) error

	// MarkInviterNotified flags that the response was delivered to the inviter.
	MarkInviterNotified(memberID string) error

	// ListAcceptedMembers returns all accepted members of a group.
	ListAcceptedMembers(groupID string) ([]Member, error)

	// ListUserGroups returns all groups in which userID is an accepted member.
	ListUserGroups(userID string) ([]Group, error)

	// ListUndeliveredInvites returns memberships where the invitee has not yet
	// been notified (InviteeNotified=false, Status=invited).
	ListUndeliveredInvites(userID string) ([]Member, error)

	// ListUndeliveredResponses returns memberships where the inviter has not
	// yet been notified of the response (InviterNotified=false, Status!=invited).
	ListUndeliveredResponses(inviterID string) ([]Member, error)

	// RemoveMember permanently deletes the membership for userID in groupID.
	// Returns ErrNotFound if no accepted membership exists.
	RemoveMember(groupID, userID string) error
}

var ErrHandleExists = errors.New("groups: handle already exists")
var ErrAlreadyMember = errors.New("groups: already a member")
var ErrNotFound = errors.New("groups: not found")

package db

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/prasenjit-net/pigeon/internal/groups"
	"github.com/prasenjit-net/pigeon/internal/queue"
)

type gormGroupStore struct {
	db *gorm.DB
}

// NewGORMGroupStore returns a groups.Store backed by the given *gorm.DB.
func NewGORMGroupStore(db *gorm.DB) groups.Store {
	return &gormGroupStore{db: db}
}

func (s *gormGroupStore) Create(ownerID, handle, name string) (groups.Group, error) {
	groupID := queue.NewID()
	g := Group{ID: groupID, Handle: handle, Name: name, OwnerID: ownerID}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&g).Error; err != nil {
			return err
		}
		ownerMember := GroupMember{
			ID:              queue.NewID(),
			GroupID:         groupID,
			UserID:          ownerID,
			InviterID:       ownerID,
			Status:          groups.Accepted,
			InviteeNotified: true,
			InviterNotified: true,
		}
		return tx.Create(&ownerMember).Error
	})
	if err != nil {
		if isUniqueConstraintError(err) {
			return groups.Group{}, groups.ErrHandleExists
		}
		return groups.Group{}, fmt.Errorf("groups: create: %w", err)
	}
	return toGroup(g), nil
}

func (s *gormGroupStore) GetByID(id string) (groups.Group, error) {
	var row Group
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return groups.Group{}, groups.ErrNotFound
		}
		return groups.Group{}, fmt.Errorf("groups: get by id: %w", err)
	}
	return toGroup(row), nil
}

func (s *gormGroupStore) GetByHandle(handle string) (groups.Group, error) {
	var row Group
	if err := s.db.First(&row, "handle = ?", handle).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return groups.Group{}, groups.ErrNotFound
		}
		return groups.Group{}, fmt.Errorf("groups: get by handle: %w", err)
	}
	return toGroup(row), nil
}

func (s *gormGroupStore) AddMember(groupID, userID, inviterID string) (groups.Member, error) {
	row := GroupMember{
		ID:        queue.NewID(),
		GroupID:   groupID,
		UserID:    userID,
		InviterID: inviterID,
		Status:    groups.Invited,
	}
	if err := s.db.Create(&row).Error; err != nil {
		if isUniqueConstraintError(err) {
			return groups.Member{}, groups.ErrAlreadyMember
		}
		return groups.Member{}, fmt.Errorf("groups: add member: %w", err)
	}
	return toMember(row), nil
}

func (s *gormGroupStore) GetMemberByID(id string) (groups.Member, error) {
	var row GroupMember
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return groups.Member{}, groups.ErrNotFound
		}
		return groups.Member{}, fmt.Errorf("groups: get member by id: %w", err)
	}
	return toMember(row), nil
}

func (s *gormGroupStore) GetMember(groupID, userID string) (groups.Member, error) {
	var row GroupMember
	if err := s.db.First(&row, "group_id = ? AND user_id = ?", groupID, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return groups.Member{}, groups.ErrNotFound
		}
		return groups.Member{}, fmt.Errorf("groups: get member: %w", err)
	}
	return toMember(row), nil
}

func (s *gormGroupStore) UpdateMemberStatus(memberID, status string) error {
	res := s.db.Model(&GroupMember{}).Where("id = ?", memberID).Update("status", status)
	if res.Error != nil {
		return fmt.Errorf("groups: update member status: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return groups.ErrNotFound
	}
	return nil
}

func (s *gormGroupStore) MarkInviteeNotified(memberID string) error {
	return s.db.Model(&GroupMember{}).Where("id = ?", memberID).Update("invitee_notified", true).Error
}

func (s *gormGroupStore) MarkInviterNotified(memberID string) error {
	return s.db.Model(&GroupMember{}).Where("id = ?", memberID).Update("inviter_notified", true).Error
}

func (s *gormGroupStore) ListAcceptedMembers(groupID string) ([]groups.Member, error) {
	var rows []GroupMember
	err := s.db.Where("group_id = ? AND status = ?", groupID, groups.Accepted).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("groups: list accepted members: %w", err)
	}
	return toMembers(rows), nil
}

func (s *gormGroupStore) ListUserGroups(userID string) ([]groups.Group, error) {
	var rows []Group
	err := s.db.
		Joins("JOIN group_members ON group_members.group_id = groups.id").
		Where("group_members.user_id = ? AND group_members.status = ?", userID, groups.Accepted).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("groups: list user groups: %w", err)
	}
	return toGroups(rows), nil
}

func (s *gormGroupStore) ListUndeliveredInvites(userID string) ([]groups.Member, error) {
	var rows []GroupMember
	err := s.db.Where(
		"user_id = ? AND status = ? AND invitee_notified = ?",
		userID, groups.Invited, false,
	).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("groups: list undelivered invites: %w", err)
	}
	return toMembers(rows), nil
}

func (s *gormGroupStore) ListUndeliveredResponses(inviterID string) ([]groups.Member, error) {
	var rows []GroupMember
	err := s.db.Where(
		"inviter_id = ? AND status != ? AND inviter_notified = ?",
		inviterID, groups.Invited, false,
	).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("groups: list undelivered responses: %w", err)
	}
	return toMembers(rows), nil
}

// compile-time assertion
var _ groups.Store = (*gormGroupStore)(nil)

func toGroup(row Group) groups.Group {
	return groups.Group{ID: row.ID, Handle: row.Handle, Name: row.Name, OwnerID: row.OwnerID}
}

func toGroups(rows []Group) []groups.Group {
	out := make([]groups.Group, len(rows))
	for i, r := range rows {
		out[i] = toGroup(r)
	}
	return out
}

func toMember(row GroupMember) groups.Member {
	return groups.Member{
		ID:        row.ID,
		GroupID:   row.GroupID,
		UserID:    row.UserID,
		InviterID: row.InviterID,
		Status:    row.Status,
	}
}

func toMembers(rows []GroupMember) []groups.Member {
	out := make([]groups.Member, len(rows))
	for i, r := range rows {
		out[i] = toMember(r)
	}
	return out
}

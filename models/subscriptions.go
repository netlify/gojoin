package models

import (
	"time"

	"errors"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"
)

type Subscription struct {
	ID   string `gorm:"unique;primary"`
	Type string

	User     *User  `json:"user,omitempty"`
	UserID   string `json:"user_id,omitempty"`
	RemoteID string
	Plan     string

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (s *Subscription) BeforeCreate(scope *gorm.Scope) error {
	s.ID = uuid.NewRandom().String()
	scope.SetColumn("ID", s.ID)

	fields := map[string]string{
		"user_id":   s.UserID,
		"plan":      s.Plan,
		"remote_id": s.RemoteID,
		"type":      s.Type,
	}

	members := []string{}
	for k, v := range fields {
		if v == "" {
			members = append(members, k)
		}
	}
	if len(members) > 0 {
		return errors.New("Missing required fields: " + strings.Join(members, ","))
	}

	return nil
}

func (Subscription) TableName() string {
	return tableName("subscriptions")
}

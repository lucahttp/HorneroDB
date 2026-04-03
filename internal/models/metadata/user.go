package metadata

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID                  string    `gorm:"type:varchar(255);primaryKey" json:"id"` // PocketID Subject (UUID string)
	Email               string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	Name                string    `gorm:"type:varchar(255)" json:"name"`
	Picture             string    `gorm:"type:text" json:"picture"`
	CanCreateWorkspaces bool      `gorm:"default:false" json:"can_create_workspaces"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	LastLoginAt         time.Time `json:"last_login_at"`
}

func (u *User) TableName() string {
	return "_hornero_users"
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	return
}

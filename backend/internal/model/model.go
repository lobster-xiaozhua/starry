package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Email        string    `gorm:"size:255;uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"size:100;not null" json:"-"`
	Role         string    `gorm:"size:16;not null;default:user" json:"role"`
	Status       string    `gorm:"size:16;not null;default:active" json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type AuthSettings struct {
	ID                     int       `gorm:"primaryKey" json:"-"`
	PasswordMinLength      int       `json:"passwordMinLength"`
	PasswordRequireUpper   bool      `json:"passwordRequireUpper"`
	PasswordRequireLower   bool      `json:"passwordRequireLower"`
	PasswordRequireDigit   bool      `json:"passwordRequireDigit"`
	PasswordRequireSpecial bool      `json:"passwordRequireSpecial"`
	PasswordMinCategories  int       `json:"passwordMinCategories"`
	LockoutThreshold       int       `json:"lockoutThreshold"`
	LockoutDurationMinutes int       `json:"lockoutDurationMinutes"`
	AccessTokenMinutes     int       `json:"accessTokenMinutes"`
	RefreshTokenDays       int       `json:"refreshTokenDays"`
	CaptchaEnabled         bool      `json:"captchaEnabled"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

func (AuthSettings) TableName() string { return "auth_settings" }

func DefaultAuthSettings() AuthSettings {
	return AuthSettings{
		ID:                     1,
		PasswordMinLength:      8,
		PasswordRequireUpper:   true,
		PasswordRequireLower:   true,
		PasswordRequireDigit:   true,
		PasswordRequireSpecial: false,
		PasswordMinCategories:  3,
		LockoutThreshold:       5,
		LockoutDurationMinutes: 15,
		AccessTokenMinutes:     15,
		RefreshTokenDays:       7,
		CaptchaEnabled:         true,
	}
}

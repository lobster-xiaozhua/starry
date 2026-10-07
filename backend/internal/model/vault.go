package model

import (
	"time"

	"github.com/google/uuid"
)

// VaultKey 是每个用户的保险箱主密钥材料（零知识）：
// Salt 与 Verifier 均为密文/非机密数据，可安全存储；主密码本身永不上传、永不落库。
// Verifier 是客户端用主密码派生密钥加密的已知常量，用于解锁时校验主密码是否正确。
type VaultKey struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	UserID    uuid.UUID `gorm:"type:uuid;uniqueIndex" json:"-"`
	Salt      string    `gorm:"size:64;not null;default:''" json:"-"`
	Verifier  string    `gorm:"type:text" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (VaultKey) TableName() string { return "vault_keys" }

// VaultItem 是一条保险箱密项：Title/Type 为明文元数据（便于列表展示），
// Encrypted 为客户端 AES-GCM 加密后的密文（服务端无法解读）。
type VaultItem struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"-"`
	Title     string    `gorm:"size:256;not null" json:"title"`
	Type      string    `gorm:"size:16;not null;default:'password'" json:"type"` // password|note|card|apikey
	Encrypted string    `gorm:"type:text;not null" json:"encrypted"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (VaultItem) TableName() string { return "vault_items" }

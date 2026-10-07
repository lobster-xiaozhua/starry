package model

import (
	"time"

	"github.com/google/uuid"
)

// DriveFile 表示网盘中的一个条目：可以是文件夹（IsDir=true）或文件。
// 文件夹树通过 ParentID 自引用实现；根目录的 ParentID 为 NULL。
type DriveFile struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID  `gorm:"type:uuid;index" json:"-"`
	ParentID   *uuid.UUID `gorm:"type:uuid;index" json:"parentId"`
	Name       string     `gorm:"size:256;not null" json:"name"`
	Mime       string     `gorm:"size:128;not null;default:'application/octet-stream'" json:"mime"`
	Size       int64      `json:"size"`
	StoredName string     `gorm:"size:128" json:"-"` // 磁盘文件名（文件夹为空）
	IsDir      bool       `gorm:"not null;default:false" json:"isDir"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

func (DriveFile) TableName() string { return "drive_files" }

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

// DriveQuota 是每个用户的网盘配额计数行，作为「已用空间」的唯一权威来源。
//
// 以往 used 由 SUM(drive_files.size) 实时派生，且上传走「先查后写」（TOCTOU），
// 并发上传会同时越过配额检查。改为维护显式计数行后，占用与释放都通过单条
// 原子的 UPDATE ... WHERE used+?<=quota / used-? 完成，数据库层面串行化，
// 既消除竞态，也保证配额计数与文件行始终一致（无需回写对账）。
type DriveQuota struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	Used      int64     `gorm:"not null;default:0" json:"-"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (DriveQuota) TableName() string { return "drive_quota" }

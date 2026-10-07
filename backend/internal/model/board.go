package model

import (
	"time"

	"github.com/google/uuid"
)

// Board 表示一个看板/项目，归属于单个用户。
type Board struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"-"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Color     string    `gorm:"size:16;not null;default:'#22c55e'" json:"color"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Board) TableName() string { return "boards" }

// BoardColumn 是看板内的一列（如 待办/进行中/已完成）。
type BoardColumn struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	BoardID   uuid.UUID `gorm:"type:uuid;index" json:"boardId"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"-"`
	Title     string    `gorm:"size:64;not null" json:"title"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (BoardColumn) TableName() string { return "board_columns" }

// BoardTask 是看板中的一张任务卡片。
type BoardTask struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"-"`
	BoardID   uuid.UUID `gorm:"type:uuid;index" json:"boardId"`
	ColumnID  uuid.UUID `gorm:"type:uuid;index" json:"columnId"`
	Title     string    `gorm:"size:256;not null" json:"title"`
	Note      string    `gorm:"type:text;not null;default:''" json:"note"`
	Priority  string    `gorm:"size:16;not null;default:'medium'" json:"priority"` // low|medium|high
	DueDate   *time.Time `json:"dueDate,omitempty"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (BoardTask) TableName() string { return "board_tasks" }

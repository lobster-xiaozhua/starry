package model

import (
	"time"

	"github.com/google/uuid"
)

// Conversation 记录一个用户的 AI 对话会话。
type Conversation struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"-"`
	Title     string    `gorm:"size:256;not null;default:'新对话'" json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Conversation) TableName() string { return "conversations" }

// Message 记录对话中的单条消息，覆盖 user / assistant / tool 三种角色。
// ToolCalls 存储 OpenAI 格式的 tool_calls JSON 字符串（assistant 消息），ToolCallID 用于 tool 消息关联。
type Message struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ConversationID uuid.UUID `gorm:"type:uuid;index" json:"conversationId"`
	UserID         uuid.UUID `gorm:"type:uuid;index" json:"-"`
	Role           string    `gorm:"size:16;not null" json:"role"`
	Content        string    `gorm:"type:text;not null;default:''" json:"content"`
	ToolCalls      string    `gorm:"type:text" json:"toolCalls,omitempty"`
	ToolCallID     string    `gorm:"size:128" json:"toolCallId,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (Message) TableName() string { return "messages" }

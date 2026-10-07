package chat

import (
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立 AI 对话模块拥有的表：会话、消息与长程任务。
func Migrate(db *store.DB) error {
	return db.AutoMigrate(&model.Conversation{}, &model.Message{}, &model.AgentTask{})
}

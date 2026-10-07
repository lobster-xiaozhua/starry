package chat

import (
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立 AI 对话模块拥有的表：会话、消息与长程任务。
//
// 除 AutoMigrate 之外，这里补齐两条复合索引：会话列表与消息回放都是
// 「按用户/会话过滤 + 按时间排序」的固定查询形态，单列索引无法同时满足过滤与排序，
// 数据量上来后会退化成「过滤后内存排序」。
func Migrate(db *store.DB) error {
	if err := db.AutoMigrate(&model.Conversation{}, &model.Message{}, &model.AgentTask{}); err != nil {
		return err
	}
	return db.ExecDDL(
		"CREATE INDEX IF NOT EXISTS idx_conversations_user_updated ON conversations(user_id, updated_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_messages_conversation_created ON messages(conversation_id, created_at)",
		"CREATE INDEX IF NOT EXISTS idx_agent_tasks_user_created ON agent_tasks(user_id, created_at DESC)",
	)
}

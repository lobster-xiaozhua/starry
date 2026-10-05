package store

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"login-system/backend/internal/model"
)

// CreateConversation 新建一个用户对话会话。
func (s *DB) CreateConversation(c *model.Conversation) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	return s.gorm.Create(c).Error
}

// ListConversations 返回用户的对话列表（最近活跃在前）。
func (s *DB) ListConversations(userID uuid.UUID) ([]model.Conversation, error) {
	var convs []model.Conversation
	err := s.gorm.Where("user_id = ?", userID).
		Order("updated_at DESC").Limit(200).Find(&convs).Error
	return convs, err
}

// GetConversation 按 ID 查询单个对话，校验用户归属。
func (s *DB) GetConversation(userID, convID uuid.UUID) (*model.Conversation, error) {
	var c model.Conversation
	err := s.gorm.Where("id = ? AND user_id = ?", convID, userID).First(&c).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// DeleteConversation 删除对话及其全部消息。
func (s *DB) DeleteConversation(userID, convID uuid.UUID) error {
	return s.gorm.Transaction(func(tx *gorm.DB) error {
		var conversation model.Conversation
		if err := tx.Where("id = ? AND user_id = ?", convID, userID).First(&conversation).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		if err := tx.Where("conversation_id = ? AND user_id = ?", convID, userID).Delete(&model.Message{}).Error; err != nil {
			return err
		}
		return tx.Delete(&conversation).Error
	})
}

// UpdateConversationTitle 更新对话标题。
func (s *DB) UpdateConversationTitle(userID, convID uuid.UUID, title string) error {
	return s.gorm.Model(&model.Conversation{}).
		Where("id = ? AND user_id = ?", convID, userID).
		Updates(map[string]interface{}{"title": title, "updated_at": time.Now()}).Error
}

// TouchConversation 更新对话的 updatedAt 时间戳（有新消息时调用）。
func (s *DB) TouchConversation(convID uuid.UUID) error {
	return s.gorm.Model(&model.Conversation{}).
		Where("id = ?", convID).
		Update("updated_at", time.Now()).Error
}

// SaveMessage 落库单条消息。
func (s *DB) SaveMessage(m *model.Message) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	return s.gorm.Create(m).Error
}

// ListMessages 返回指定对话的全部消息，按时间正序。
func (s *DB) ListMessages(userID, convID uuid.UUID) ([]model.Message, error) {
	var msgs []model.Message
	err := s.gorm.Where("conversation_id = ? AND user_id = ?", convID, userID).
		Order("created_at ASC").Find(&msgs).Error
	return msgs, err
}

// ListRecentMessages 返回指定对话最近 limit 条消息，按时间正序。
// limit <= 0 时返回全量。用于 agent 回放历史，避免一次性加载整段对话。
func (s *DB) ListRecentMessages(userID, convID uuid.UUID, limit int) ([]model.Message, error) {
	var msgs []model.Message
	q := s.gorm.Where("conversation_id = ? AND user_id = ?", convID, userID).
		Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&msgs).Error; err != nil {
		return nil, err
	}
	// DESC 取最近 N 条后反转为时间正序，便于下游直接拼接
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// DeleteMessages 批量删除对话中的消息（按 ID），校验用户归属。
// 供 agent 摘要后清理已被摘要替代的原始消息，保持对话存储精简。
func (s *DB) DeleteMessages(userID, convID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	return s.gorm.Where("conversation_id = ? AND user_id = ? AND id IN ?", convID, userID, ids).
		Delete(&model.Message{}).Error
}

// truncateRunes 按 rune（字符）数截断字符串，避免在多字节字符中间切断。
func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "\n…(truncated)"
}

// CompressMessages 对历史消息做无损压缩：截断过长的 tool 结果与 assistant 正文，
// 保持消息顺序与 tool_call/tool_result 配对不变。仅用于回放历史，不影响落库数据。
func CompressMessages(msgs []model.Message, toolCap, assistantCap int) []model.Message {
	if toolCap <= 0 {
		toolCap = 2000
	}
	if assistantCap <= 0 {
		assistantCap = 8000
	}
	out := make([]model.Message, len(msgs))
	for i, m := range msgs {
		switch m.Role {
		case "tool":
			m.Content = truncateRunes(m.Content, toolCap)
		case "assistant":
			m.Content = truncateRunes(m.Content, assistantCap)
		}
		out[i] = m
	}
	return out
}

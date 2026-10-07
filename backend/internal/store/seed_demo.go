package store

import (
	"time"

	"github.com/google/uuid"

	"starry/backend/internal/model"
)

// CountNotes 返回该用户的笔记总数，用于判断是否需要灌示例数据。
func (s *DB) CountNotes(userID uuid.UUID) (int64, error) {
	var count int64
	if err := s.gorm.Model(&model.Note{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountConversations 返回该用户的对话总数。
func (s *DB) CountConversations(userID uuid.UUID) (int64, error) {
	var count int64
	if err := s.gorm.Model(&model.Conversation{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// SeedDemoNotes 为该用户灌入一组中文示例笔记（仅在尚无笔记时执行，幂等）。
// 返回实际创建的条数。
func (s *DB) SeedDemoNotes(userID uuid.UUID) (int, error) {
	count, err := s.CountNotes(userID)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		return 0, nil
	}
	demos := []struct {
		title string
		body  string
		tags  []string
	}{
		{
			title: "欢迎使用 Starry 云笔记",
			body:  "# Starry 云笔记\n\n这是你的个人知识库。**云笔记** 与 **AI 对话 / 工作模式** 深度打通：\n\n- 在「AI 对话」里随时把灵感存成笔记\n- 在「工作模式」让 Agent 读取并整理你的笔记\n- 用 `#标签` 给笔记分类，支持全文搜索\n\n试着新建一条笔记，或让 AI 帮你总结已有内容。",
			tags:  []string{"入门", "指南"},
		},
		{
			title: "AI 工作模式能做什么",
			body:  "## 两种 AI 模式\n\n- **AI 对话（日常）**：轻量问答，适合随手咨询、写文案、解释概念。\n- **工作模式（Agent）**：内置笔记工具，可检索、读取你的笔记并据此完成任务。\n\n> 提示：工作模式依赖大模型密钥（LLM_API_KEY），由部署者配置。",
			tags:  []string{"ai", "效率"},
		},
		{
			title: "周会纪要模板",
			body:  "## 周会纪要\n\n**时间**：\n**参会**：\n\n### 本周进展\n1. \n2. \n\n### 风险与阻塞\n- \n\n### 下周计划\n1. \n2. ",
			tags:  []string{"模板", "工作"},
		},
	}
	now := time.Now()
	n := 0
	for _, d := range demos {
		note := &model.Note{
			UserID:    userID,
			Title:     d.title,
			Body:      d.body,
			CreatedAt: now.Add(time.Duration(-n) * time.Minute),
			UpdatedAt: now.Add(time.Duration(-n) * time.Minute),
		}
		if err := s.CreateNote(note); err != nil {
			return n, err
		}
		if err := s.LinkNoteTags(note.ID, userID, d.tags); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// SeedDemoConversation 为该用户灌入一条示例对话（仅在尚无对话时执行，幂等）。
// 返回实际创建的对话数（0 或 1）。
func (s *DB) SeedDemoConversation(userID uuid.UUID) (int, error) {
	count, err := s.CountConversations(userID)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		return 0, nil
	}
	conv := &model.Conversation{
		UserID: userID,
		Title:  "示例：用 AI 整理今日待办",
	}
	if err := s.CreateConversation(conv); err != nil {
		return 0, err
	}
	now := time.Now()
	msgs := []model.Message{
		{
			ConversationID: conv.ID,
			UserID:         userID,
			Role:           "user",
			Content:        "帮我把今天的工作整理成三件最重要的事，并给出执行建议。",
			CreatedAt:      now,
		},
		{
			ConversationID: conv.ID,
			UserID:         userID,
			Role:           "assistant",
			Content:        "好的，这里是基于通用工作方法的建议（实际效果取决于你的笔记与工作模式配置）：\n\n1. **推进关键里程碑**——挑一个最影响进度的任务优先做。\n2. **清理阻塞项**——列出依赖并推动相关方。\n3. **沉淀今日产出**——用一条云笔记记录结论，便于明天延续。\n\n需要的话，我可以读取你的笔记并据此细化。",
			CreatedAt:      now.Add(2 * time.Second),
		},
	}
	for i := range msgs {
		if err := s.SaveMessage(&msgs[i]); err != nil {
			return 0, err
		}
	}
	return 1, nil
}

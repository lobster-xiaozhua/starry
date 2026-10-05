package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"login-system/backend/internal/model"
	"login-system/backend/internal/store"
)

// AgentHandler 处理 AI 对话的 CRUD 端点。流式推理由 Node agent 服务承担，
// 此 handler 仅负责对话与消息的持久化读写。
type AgentHandler struct {
	db  *store.DB
	rds *store.Redis
}

func NewAgentHandler(db *store.DB, rds *store.Redis) *AgentHandler {
	return &AgentHandler{db: db, rds: rds}
}

func (h *AgentHandler) userID(c *gin.Context) (uuid.UUID, bool) {
	id, _ := c.Get("userID")
	v, ok := id.(string)
	if !ok {
		return uuid.Nil, false
	}
	u, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, false
	}
	return u, true
}

// ListConversations GET /api/agent/conversations
func (h *AgentHandler) ListConversations(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	convs, err := h.db.ListConversations(uid)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	OK(c, gin.H{"conversations": convs})
}

// CreateConversation POST /api/agent/conversations
func (h *AgentHandler) CreateConversation(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	var in struct {
		Title string `json:"title"`
	}
	_ = c.ShouldBindJSON(&in)
	title := in.Title
	if title == "" {
		title = "新对话"
	}
	conv := &model.Conversation{UserID: uid, Title: title}
	if err := h.db.CreateConversation(conv); err != nil {
		Fail(c, http.StatusInternalServerError, 3003, err.Error())
		return
	}
	OK(c, conv)
}

// DeleteConversation DELETE /api/agent/conversations/:id
func (h *AgentHandler) DeleteConversation(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, http.StatusBadRequest, 3004, "invalid conversation id")
		return
	}
	if err := h.db.DeleteConversation(uid, convID); err != nil {
		Fail(c, http.StatusInternalServerError, 3005, err.Error())
		return
	}
	OK(c, gin.H{"id": convID.String()})
}

// ListMessages GET /api/agent/conversations/:id/messages
func (h *AgentHandler) ListMessages(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, http.StatusBadRequest, 3004, "invalid conversation id")
		return
	}
	conv, err := h.db.GetConversation(uid, convID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	if conv == nil {
		Fail(c, http.StatusNotFound, 3006, "conversation not found")
		return
	}
	// 默认取最近 50 条并压缩，避免一次性回放整段对话导致 token 爆炸。
	// 裁剪与压缩均在 Go 数据层完成；摘要逻辑留在 Node。
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 500 {
				n = 500
			}
			limit = n
		}
	}
	compress := c.DefaultQuery("compress", "1")
	doCompress := compress == "1" || compress == "true"

	msgs, err := h.db.ListRecentMessages(uid, convID, limit)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 3007, err.Error())
		return
	}
	if doCompress {
		msgs = store.CompressMessages(msgs, 0, 0)
	}
	OK(c, gin.H{"messages": msgs})
}

// SaveMessage POST /api/agent/conversations/:id/messages
func (h *AgentHandler) SaveMessage(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, http.StatusBadRequest, 3004, "invalid conversation id")
		return
	}
	conv, err := h.db.GetConversation(uid, convID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	if conv == nil {
		Fail(c, http.StatusNotFound, 3006, "conversation not found")
		return
	}
	var in struct {
		Role       string `json:"role" binding:"required"`
		Content    string `json:"content"`
		ToolCalls  string `json:"toolCalls"`
		ToolCallID string `json:"toolCallId"`
		CreatedAt  string `json:"createdAt"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, 3008, "invalid body: "+err.Error())
		return
	}
	// summary 角色由 Node 摘要流程写入，作为后续轮次的历史上下文。
	if in.Role != "user" && in.Role != "assistant" && in.Role != "tool" && in.Role != "summary" {
		Fail(c, http.StatusBadRequest, 3008, "invalid message role")
		return
	}
	msg := &model.Message{
		ConversationID: convID,
		UserID:         uid,
		Role:           in.Role,
		Content:        in.Content,
		ToolCalls:      in.ToolCalls,
		ToolCallID:     in.ToolCallID,
	}
	if in.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, in.CreatedAt); err == nil {
			msg.CreatedAt = t
		}
	}
	if err := h.db.SaveMessage(msg); err != nil {
		Fail(c, http.StatusInternalServerError, 3009, err.Error())
		return
	}
	_ = h.db.TouchConversation(convID)
	OK(c, msg)
}

// DeleteMessages POST /api/agent/conversations/:id/messages/delete
// 批量删除消息，供 agent 摘要后清理已被摘要替代的原始消息，保持存储精简。
func (h *AgentHandler) DeleteMessages(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, http.StatusBadRequest, 3004, "invalid conversation id")
		return
	}
	conv, err := h.db.GetConversation(uid, convID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	if conv == nil {
		Fail(c, http.StatusNotFound, 3006, "conversation not found")
		return
	}
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, 3008, "invalid body: "+err.Error())
		return
	}
	ids := make([]uuid.UUID, 0, len(in.IDs))
	for _, s := range in.IDs {
		if u, err := uuid.Parse(s); err == nil {
			ids = append(ids, u)
		}
	}
	if err := h.db.DeleteMessages(uid, convID, ids); err != nil {
		Fail(c, http.StatusInternalServerError, 3010, err.Error())
		return
	}
	OK(c, gin.H{"deleted": len(ids)})
}

// RecordUsage POST /api/agent/usage
// 累加用户本轮 LLM token 用量到 Redis。Node agent 在每轮 LLM 调用后上报。
func (h *AgentHandler) RecordUsage(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	var in struct {
		PromptTokens     int64  `json:"promptTokens"`
		CompletionTokens int64  `json:"completionTokens"`
		Model            string `json:"model"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, 3008, "invalid body: "+err.Error())
		return
	}
	if err := h.rds.IncrUsage(c.Request.Context(), uid.String(), in.PromptTokens, in.CompletionTokens); err != nil {
		Fail(c, http.StatusInternalServerError, 3010, err.Error())
		return
	}
	OK(c, gin.H{"recorded": true})
}

// GetUsage GET /api/agent/usage
// 返回用户累计的 prompt/completion token 用量。
func (h *AgentHandler) GetUsage(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	prompt, completion, err := h.rds.GetUsage(c.Request.Context(), uid.String())
	if err != nil {
		Fail(c, http.StatusInternalServerError, 3010, err.Error())
		return
	}
	OK(c, gin.H{"promptTokens": prompt, "completionTokens": completion})
}

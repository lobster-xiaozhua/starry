package vault

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"starry/backend/internal/core"
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Handler 处理密码/密钥保险箱。服务端只持久化密文与密钥材料，不做任何加解密。
type Handler struct {
	db *store.DB
}

func New(db *store.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) userID(c *gin.Context) (uuid.UUID, bool) {
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

func (h *Handler) parseID(c *gin.Context, param string) (uuid.UUID, bool) {
	raw := c.Param(param)
	id, err := uuid.Parse(raw)
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 4000, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

// GetSalt GET /api/vault/salt  → {salt, verifier}
// 返回现有密钥材料；若无则生成随机 salt 并落库后返回（salt 非机密）。
func (h *Handler) GetSalt(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	salt, verifier, err := h.db.EnsureVaultSalt(uid)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"salt": salt, "verifier": verifier, "setup": verifier != ""})
}

// Setup POST /api/vault/setup  {verifier}
// 首次设定主密码：客户端用主密码派生密钥加密已知常量，把密文（verifier）写入。
func (h *Handler) Setup(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	var body struct {
		Verifier string `json:"verifier" binding:"max=262144"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Verifier) == "" {
		core.Fail(c, http.StatusBadRequest, 4001, "verifier required")
		return
	}
	if err := h.db.SaveVaultVerifier(uid, body.Verifier); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"ok": true})
}

// ListItems GET /api/vault/items  → {items: VaultItem[]}（含密文）
func (h *Handler) ListItems(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	items, err := h.db.ListVaultItems(uid)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	if items == nil {
		items = []model.VaultItem{}
	}
	core.OK(c, gin.H{"items": items})
}

// CreateItem POST /api/vault/items  {title, type, encrypted}
func (h *Handler) CreateItem(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	var body struct {
		Title     string `json:"title" binding:"max=200"`
		Type      string `json:"type" binding:"max=32"`
		Encrypted string `json:"encrypted" binding:"max=262144"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Title) == "" || strings.TrimSpace(body.Encrypted) == "" {
		core.Fail(c, http.StatusBadRequest, 4001, "title and encrypted required")
		return
	}
	if body.Type == "" {
		body.Type = "password"
	}
	item := &model.VaultItem{
		UserID:    uid,
		Title:     strings.TrimSpace(body.Title),
		Type:      body.Type,
		Encrypted: body.Encrypted,
	}
	if err := h.db.CreateVaultItem(item); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, item)
}

// UpdateItem PATCH /api/vault/items/:id  {title?, encrypted?}
func (h *Handler) UpdateItem(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	id, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	var body struct {
		Title     *string `json:"title" binding:"max=200"`
		Encrypted *string `json:"encrypted" binding:"max=262144"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		core.Fail(c, http.StatusBadRequest, 4001, "invalid body")
		return
	}
	if err := h.db.UpdateVaultItem(uid, id, body.Title, body.Encrypted); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"ok": true})
}

// DeleteItem DELETE /api/vault/items/:id
func (h *Handler) DeleteItem(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	id, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	if err := h.db.DeleteVaultItem(uid, id); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"ok": true})
}

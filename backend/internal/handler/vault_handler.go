package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"login-system/backend/internal/model"
	"login-system/backend/internal/store"
)

// VaultHandler 处理密码/密钥保险箱。服务端只持久化密文与密钥材料，不做任何加解密。
type VaultHandler struct {
	db *store.DB
}

func NewVaultHandler(db *store.DB) *VaultHandler {
	return &VaultHandler{db: db}
}

func (h *VaultHandler) userID(c *gin.Context) (uuid.UUID, bool) {
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

func (h *VaultHandler) parseID(c *gin.Context, param string) (uuid.UUID, bool) {
	raw := c.Param(param)
	id, err := uuid.Parse(raw)
	if err != nil {
		Fail(c, http.StatusBadRequest, 4000, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

// GetSalt GET /api/vault/salt  → {salt, verifier}
// 返回现有密钥材料；若无则生成随机 salt 并落库后返回（salt 非机密）。
func (h *VaultHandler) GetSalt(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	salt, verifier, err := h.db.EnsureVaultSalt(uid)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	OK(c, gin.H{"salt": salt, "verifier": verifier, "setup": verifier != ""})
}

// Setup POST /api/vault/setup  {verifier}
// 首次设定主密码：客户端用主密码派生密钥加密已知常量，把密文（verifier）写入。
func (h *VaultHandler) Setup(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	var body struct {
		Verifier string `json:"verifier"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Verifier) == "" {
		Fail(c, http.StatusBadRequest, 4001, "verifier required")
		return
	}
	if err := h.db.SaveVaultVerifier(uid, body.Verifier); err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	OK(c, gin.H{"ok": true})
}

// ListItems GET /api/vault/items  → {items: VaultItem[]}（含密文）
func (h *VaultHandler) ListItems(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	items, err := h.db.ListVaultItems(uid)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	if items == nil {
		items = []model.VaultItem{}
	}
	OK(c, gin.H{"items": items})
}

// CreateItem POST /api/vault/items  {title, type, encrypted}
func (h *VaultHandler) CreateItem(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	var body struct {
		Title     string `json:"title"`
		Type      string `json:"type"`
		Encrypted string `json:"encrypted"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Title) == "" || strings.TrimSpace(body.Encrypted) == "" {
		Fail(c, http.StatusBadRequest, 4001, "title and encrypted required")
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
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	OK(c, item)
}

// UpdateItem PATCH /api/vault/items/:id  {title?, encrypted?}
func (h *VaultHandler) UpdateItem(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	id, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	var body struct {
		Title     *string `json:"title"`
		Encrypted *string `json:"encrypted"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, http.StatusBadRequest, 4001, "invalid body")
		return
	}
	if err := h.db.UpdateVaultItem(uid, id, body.Title, body.Encrypted); err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	OK(c, gin.H{"ok": true})
}

// DeleteItem DELETE /api/vault/items/:id
func (h *VaultHandler) DeleteItem(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	id, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	if err := h.db.DeleteVaultItem(uid, id); err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	OK(c, gin.H{"ok": true})
}

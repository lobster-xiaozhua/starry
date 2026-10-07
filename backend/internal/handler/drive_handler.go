package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"login-system/backend/internal/config"
	"login-system/backend/internal/model"
	"login-system/backend/internal/store"
)

// DriveHandler 处理网盘的文件/文件夹管理，含配额校验与递归删除。
type DriveHandler struct {
	cfg   *config.Config
	db    *store.DB
	drive *store.DriveStore
}

func NewDriveHandler(cfg *config.Config, db *store.DB, drive *store.DriveStore) *DriveHandler {
	return &DriveHandler{cfg: cfg, db: db, drive: drive}
}

func (h *DriveHandler) userID(c *gin.Context) (uuid.UUID, bool) {
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

func (h *DriveHandler) parseID(c *gin.Context, param string) (uuid.UUID, bool) {
	raw := c.Param(param)
	id, err := uuid.Parse(raw)
	if err != nil {
		Fail(c, http.StatusBadRequest, 4000, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

// parseParent 解析 parent 参数："root" 或 NULL；否则按 uuid 解析。
func (h *DriveHandler) parseParent(c *gin.Context, raw string) (*uuid.UUID, bool) {
	if raw == "" || raw == "root" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		Fail(c, http.StatusBadRequest, 4000, "invalid parent")
		return nil, false
	}
	return &id, true
}

// List GET /api/drive?parent=<id|root>
func (h *DriveHandler) List(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	parent, ok := h.parseParent(c, c.Query("parent"))
	if !ok {
		return
	}
	items, err := h.db.ListDriveChildren(uid, parent)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	used, err := h.db.SumDriveUsage(uid)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	if items == nil {
		items = []model.DriveFile{}
	}
	OK(c, gin.H{
		"items":  items,
		"used":   used,
		"quota":  h.cfg.DriveQuotaBytes,
		"maxOne": h.cfg.DriveMaxBytes,
	})
}

// CreateFolder POST /api/drive/folders  {name, parentID?}
func (h *DriveHandler) CreateFolder(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	var body struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parentID"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		Fail(c, http.StatusBadRequest, 4001, "name required")
		return
	}
	var parentID *uuid.UUID
	if body.ParentID != nil && *body.ParentID != "" && *body.ParentID != "root" {
		if p, err := uuid.Parse(*body.ParentID); err == nil {
			parentID = &p
		}
	}
	f, err := h.db.CreateDriveFolder(uid, parentID, strings.TrimSpace(body.Name))
	if err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	OK(c, f)
}

// Upload POST /api/drive/upload  (multipart: file, parentID?)
func (h *DriveHandler) Upload(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, 4001, "file required")
		return
	}
	if fileHeader.Size > h.cfg.DriveMaxBytes {
		Fail(c, http.StatusBadRequest, 4002, "单文件超过大小上限")
		return
	}
	used, err := h.db.SumDriveUsage(uid)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	if used+fileHeader.Size > h.cfg.DriveQuotaBytes {
		Fail(c, http.StatusBadRequest, 4003, "网盘空间不足")
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	defer src.Close()
	storedName, err := h.drive.Save(uid, src)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	var parentID *uuid.UUID
	if p := c.PostForm("parentID"); p != "" && p != "root" {
		if pid, perr := uuid.Parse(p); perr == nil {
			parentID = &pid
		}
	}
	f := &model.DriveFile{
		UserID:     uid,
		ParentID:   parentID,
		Name:       fileHeader.Filename,
		Mime:       fileHeader.Header.Get("Content-Type"),
		Size:       fileHeader.Size,
		StoredName: storedName,
		IsDir:      false,
	}
	if f.Mime == "" {
		f.Mime = "application/octet-stream"
	}
	if err := h.db.InsertDriveFile(f); err != nil {
		h.drive.Delete(uid, storedName)
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	OK(c, f)
}

// Download GET /api/drive/:id/download
func (h *DriveHandler) Download(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	id, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	f, err := h.db.GetDriveFile(uid, id)
	if err != nil || f == nil {
		Fail(c, http.StatusNotFound, 4040, "file not found")
		return
	}
	if f.IsDir {
		Fail(c, http.StatusBadRequest, 4001, "folder cannot be downloaded")
		return
	}
	c.FileAttachment(h.drive.Path(uid, f.StoredName), f.Name)
}

// Rename PATCH /api/drive/:id  {name}
func (h *DriveHandler) Rename(c *gin.Context) {
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
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		Fail(c, http.StatusBadRequest, 4001, "name required")
		return
	}
	if err := h.db.RenameDriveFile(uid, id, strings.TrimSpace(body.Name)); err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	OK(c, gin.H{"ok": true})
}

// Delete DELETE /api/drive/:id  （文件夹递归删除）
func (h *DriveHandler) Delete(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	id, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	if err := h.deleteNode(uid, id); err != nil {
		Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	OK(c, gin.H{"ok": true})
}

// deleteNode 递归删除节点：先删子文件磁盘 + 子目录，再删自身。
func (h *DriveHandler) deleteNode(uid, id uuid.UUID) error {
	node, err := h.db.GetDriveFile(uid, id)
	if err != nil {
		return err
	}
	if node == nil {
		return nil
	}
	if node.IsDir {
		children, err := h.db.ListDriveChildren(uid, &id)
		if err != nil {
			return err
		}
		for _, ch := range children {
			if err := h.deleteNode(uid, ch.ID); err != nil {
				return err
			}
		}
	}
	if !node.IsDir {
		h.drive.Delete(uid, node.StoredName)
	}
	return h.db.DeleteDriveFileRow(uid, id)
}

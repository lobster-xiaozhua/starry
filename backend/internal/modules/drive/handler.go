package drive

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"starry/backend/internal/config"
	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// driveEnvelopeOverhead 是 multipart 封装本身的开销余量：boundary、字段名、
// 以及 parentID 等伴生表单项都要算进请求体总量，否则恰好接近上限的文件会被误拒。
const driveEnvelopeOverhead = 2 << 20

// Handler 处理网盘的文件/文件夹管理，含配额校验与递归删除。
type Handler struct {
	cfg   *config.Config
	db    *store.DB
	drive *store.DriveStore
}

func New(cfg *config.Config, db *store.DB, drive *store.DriveStore) *Handler {
	return &Handler{cfg: cfg, db: db, drive: drive}
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

// parseParent 解析 parent 参数："root" 或 NULL；否则按 uuid 解析。
func (h *Handler) parseParent(c *gin.Context, raw string) (*uuid.UUID, bool) {
	if raw == "" || raw == "root" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 4000, "invalid parent")
		return nil, false
	}
	return &id, true
}

// List GET /api/drive?parent=<id|root>
func (h *Handler) List(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	parent, ok := h.parseParent(c, c.Query("parent"))
	if !ok {
		return
	}
	items, err := h.db.ListDriveChildren(uid, parent)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	// used 来自配额计数行（drive_quota.used），是已用空间的权威来源，
	// 而非每次实时聚合，避免与上传的原子占用产生不一致。
	q, err := h.db.GetOrCreateDriveQuota(uid)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	used := q.Used
	if items == nil {
		items = []model.DriveFile{}
	}
	core.OK(c, gin.H{
		"items":  items,
		"used":   used,
		"quota":  h.cfg.DriveQuotaBytes,
		"maxOne": h.cfg.DriveMaxBytes,
	})
}

// CreateFolder POST /api/drive/folders  {name, parentID?}
func (h *Handler) CreateFolder(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	var body struct {
		Name     string  `json:"name" binding:"max=200"`
		ParentID *string `json:"parentID"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		core.Fail(c, http.StatusBadRequest, 4001, "name required")
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
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, f)
}

// Upload POST /api/drive/upload  (multipart: file, parentID?)
func (h *Handler) Upload(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	// 在解析 multipart 之前抬高请求体配额到网盘允许的量级（multipart 边界本身也要占字节，故留出余量）。
	// 用 RaiseBodyLimit 而不是自己再包一层 MaxBytesReader：后者是「叠加」而非「替换」，
	// 外层的全局默认（8MB）会先于网盘自己的额度生效，大文件上传依旧会被拦。
	middleware.RaiseBodyLimit(c, h.cfg.DriveMaxBytes+driveEnvelopeOverhead)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		if middleware.IsBodyTooLarge(err) {
			core.Fail(c, http.StatusRequestEntityTooLarge, 4002, "单文件超过大小上限")
			return
		}
		core.Fail(c, http.StatusBadRequest, 4001, "file required")
		return
	}
	if fileHeader.Size > h.cfg.DriveMaxBytes {
		core.Fail(c, http.StatusBadRequest, 4002, "单文件超过大小上限")
		return
	}
	// 配额原子占用：单条带条件 UPDATE（used+?<=quota）在数据库层面串行执行，
	// 杜绝并发上传「先查后写」的竞态（TOCTOU），永远不会越过网盘配额。
	if err := h.db.EnsureDriveQuotaRow(uid); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	claimed, err := h.db.ClaimDriveSpace(uid, fileHeader.Size, h.cfg.DriveQuotaBytes)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	if !claimed {
		core.Fail(c, http.StatusBadRequest, 4003, "网盘空间不足")
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		h.db.ReleaseDriveSpace(uid, fileHeader.Size)
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	defer src.Close()
	storedName, err := h.drive.Save(uid, src)
	if err != nil {
		h.db.ReleaseDriveSpace(uid, fileHeader.Size)
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
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
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, f)
}

// Download GET /api/drive/:id/download
func (h *Handler) Download(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	id, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	f, err := h.db.GetDriveFile(uid, id)
	if err != nil || f == nil {
		core.Fail(c, http.StatusNotFound, 4040, "file not found")
		return
	}
	if f.IsDir {
		core.Fail(c, http.StatusBadRequest, 4001, "folder cannot be downloaded")
		return
	}
	path := h.drive.Path(uid, f.StoredName)
	data, err := os.ReadFile(path)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, "读取文件失败")
		return
	}
	// 强制以附件形式下载，并使用 application/octet-stream + nosniff：
	// 阻止浏览器对上传的 .html 等内容按同源渲染，规避存储型 XSS。
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.QueryEscape(f.Name))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// Rename PATCH /api/drive/:id  {name}
func (h *Handler) Rename(c *gin.Context) {
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
		Name string `json:"name" binding:"max=200"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		core.Fail(c, http.StatusBadRequest, 4001, "name required")
		return
	}
	if err := h.db.RenameDriveFile(uid, id, strings.TrimSpace(body.Name)); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"ok": true})
}

// Delete DELETE /api/drive/:id  （文件夹递归删除）
func (h *Handler) Delete(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	id, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	if err := h.deleteNode(uid, id); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"ok": true})
}

// deleteNode 递归删除节点：先删子文件磁盘 + 子目录，再删自身。
func (h *Handler) deleteNode(uid, id uuid.UUID) error {
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
	// 删除元数据行并在同一事务内归还配额，避免「删了文件却没回写配额」的漂移。
	if _, err := h.db.DeleteDriveFileAndReclaim(uid, id); err != nil {
		return err
	}
	return nil
}

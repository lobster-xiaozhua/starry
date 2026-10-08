package boards

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"starry/backend/internal/core"
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Handler 处理看板/列/任务的 CRUD 与移动。
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

// List GET /api/boards
func (h *Handler) List(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	tree, err := h.db.ListBoardTree(uid)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	if tree == nil {
		tree = []store.BoardView{}
	}
	core.OK(c, gin.H{"boards": tree})
}

// Create POST /api/boards  {name, color?}
func (h *Handler) Create(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	var body struct {
		Name  string `json:"name" binding:"max=200"`
		Color string `json:"color" binding:"max=32"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		core.Fail(c, http.StatusBadRequest, 4001, "name required")
		return
	}
	board, err := h.db.CreateBoard(uid, strings.TrimSpace(body.Name), body.Color)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, board)
}

// Update PATCH /api/boards/:id  {name?, color?, position?}
func (h *Handler) Update(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	boardID, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	var body struct {
		Name     *string `json:"name" binding:"max=200"`
		Color    *string `json:"color" binding:"max=32"`
		Position *int    `json:"position"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		core.Fail(c, http.StatusBadRequest, 4001, "invalid body")
		return
	}
	if err := h.db.UpdateBoard(uid, boardID, body.Name, body.Color, body.Position); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"ok": true})
}

// Delete DELETE /api/boards/:id
func (h *Handler) Delete(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	boardID, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	if err := h.db.DeleteBoard(uid, boardID); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"ok": true})
}

// CreateColumn POST /api/boards/:id/columns  {title}
func (h *Handler) CreateColumn(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	boardID, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	var body struct {
		Title string `json:"title" binding:"max=200"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Title) == "" {
		core.Fail(c, http.StatusBadRequest, 4001, "title required")
		return
	}
	col, err := h.db.CreateColumn(uid, boardID, strings.TrimSpace(body.Title))
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, col)
}

// UpdateColumn PATCH /api/columns/:id  {title?, position?}
func (h *Handler) UpdateColumn(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	colID, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	var body struct {
		Title    *string `json:"title" binding:"max=200"`
		Position *int    `json:"position"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		core.Fail(c, http.StatusBadRequest, 4001, "invalid body")
		return
	}
	if err := h.db.UpdateColumn(uid, colID, body.Title, body.Position); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"ok": true})
}

// DeleteColumn DELETE /api/columns/:id
func (h *Handler) DeleteColumn(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	colID, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	if err := h.db.DeleteColumn(uid, colID); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"ok": true})
}

// CreateTask POST /api/boards/:id/tasks  {title, columnId, note?, priority?, due?}
func (h *Handler) CreateTask(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	boardID, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	var body struct {
		Title    string  `json:"title" binding:"max=200"`
		ColumnID string  `json:"columnId"`
		Note     string  `json:"note" binding:"max=8192"`
		Priority string  `json:"priority" binding:"max=32"`
		Due      *string `json:"due"` // ISO time
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Title) == "" {
		core.Fail(c, http.StatusBadRequest, 4001, "title required")
		return
	}
	colID, err := uuid.Parse(body.ColumnID)
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 4002, "invalid columnId")
		return
	}
	priority := body.Priority
	if priority == "" {
		priority = "medium"
	}
	task := &model.BoardTask{
		UserID:   uid,
		BoardID:  boardID,
		ColumnID: colID,
		Title:    strings.TrimSpace(body.Title),
		Note:     body.Note,
		Priority: priority,
	}
	if body.Due != nil && *body.Due != "" {
		if t, perr := time.Parse(time.RFC3339, *body.Due); perr == nil {
			task.DueDate = &t
		}
	}
	created, err := h.db.CreateBoardTask(task)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, created)
}

// UpdateTask PATCH /api/tasks/:id
// 可更新：title, note, priority, due, columnId, position（移动/改状态）
func (h *Handler) UpdateTask(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	taskID, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	var body struct {
		Title    *string `json:"title" binding:"max=200"`
		Note     *string `json:"note" binding:"max=8192"`
		Priority *string `json:"priority" binding:"max=32"`
		Due      *string `json:"due"`
		ColumnID *string `json:"columnId"`
		Position *int    `json:"position"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		core.Fail(c, http.StatusBadRequest, 4001, "invalid body")
		return
	}
	patch := map[string]interface{}{}
	if body.Title != nil {
		patch["title"] = strings.TrimSpace(*body.Title)
	}
	if body.Note != nil {
		patch["note"] = *body.Note
	}
	if body.Priority != nil {
		patch["priority"] = *body.Priority
	}
	if body.Position != nil {
		patch["position"] = *body.Position
	}
	if body.Due != nil {
		if *body.Due == "" {
			patch["due_date"] = nil
		} else if t, perr := time.Parse(time.RFC3339, *body.Due); perr == nil {
			patch["due_date"] = t
		}
	}
	if body.ColumnID != nil && *body.ColumnID != "" {
		if cid, cerr := uuid.Parse(*body.ColumnID); cerr == nil {
			patch["column_id"] = cid
		}
	}
	if err := h.db.UpdateTask(uid, taskID, patch); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"ok": true})
}

// DeleteTask DELETE /api/tasks/:id
func (h *Handler) DeleteTask(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	taskID, ok := h.parseID(c, "id")
	if !ok {
		return
	}
	if err := h.db.DeleteTask(uid, taskID); err != nil {
		core.Fail(c, http.StatusInternalServerError, 5000, err.Error())
		return
	}
	core.OK(c, gin.H{"ok": true})
}

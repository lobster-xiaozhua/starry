package notes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
	"starry/backend/internal/model"
	"starry/backend/internal/request"
	"starry/backend/internal/sse"
	"starry/backend/internal/store"
)

type Handler struct {
	svc    *NotesService
	media  *store.FileStore
	broker *sse.Broker
}

func New(svc *NotesService, media *store.FileStore, broker *sse.Broker) *Handler {
	return &Handler{svc: svc, media: media, broker: broker}
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

func (h *Handler) List(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	page := request.Page(c)
	size := request.Size(c, 20, 100)
	if c.Query("q") != "" {
		notes, total, err := h.svc.Search(c.Request.Context(), uid, c.Query("q"), page, size)
		if err != nil {
			core.Fail(c, http.StatusInternalServerError, 2004, err.Error())
			return
		}
		core.OK(c, gin.H{"notes": notes, "total": total, "page": page, "size": size})
		return
	}
	var arch *bool
	if v := c.Query("archived"); v != "" {
		b := v == "true"
		arch = &b
	}
	var tags []string
	if v := c.Query("tag"); v != "" {
		tags = strings.Split(v, ",")
	}
	sortSQL := request.Sort(c, map[string]string{
		"updated_at": "updated_at",
		"created_at": "created_at",
		"title":      "title",
	}, "updated_at DESC")
	notes, total, err := h.svc.List(c.Request.Context(), store.NoteListQuery{
		UserID: uid, Tags: tags, Archived: arch, Page: page, Size: size, Sort: sortSQL,
	})
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 2004, err.Error())
		return
	}
	core.OK(c, gin.H{"notes": notes, "total": total, "page": page, "size": size})
}

// SeedDemo POST /api/notes/seed-demo
// 为该用户灌入示例笔记（仅在尚无笔记时执行，幂等）。便于新用户一键体验产品。
func (h *Handler) SeedDemo(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	n, err := h.svc.SeedDemo(c.Request.Context(), uid)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 2004, err.Error())
		return
	}
	if n == 0 {
		core.OK(c, gin.H{"count": 0, "message": "已有笔记，未重复灌入"})
		return
	}
	h.publish(c, "", "note.created")
	core.OK(c, gin.H{"count": n, "message": "已为你加载示例笔记"})
}

func (h *Handler) Create(c *gin.Context) {
	uid, _ := h.userID(c)
	var in CreateNoteInput
	if err := c.ShouldBindJSON(&in); err != nil {
		core.Fail(c, http.StatusBadRequest, 2005, "invalid body: "+err.Error())
		return
	}
	note, err := h.svc.Create(c.Request.Context(), uid, in)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 2006, err.Error())
		return
	}
	h.publish(c, note.ID.String(), "note.created")
	core.OK(c, note)
}

func (h *Handler) Get(c *gin.Context) {
	uid, _ := h.userID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 2005, "invalid note id")
		return
	}
	note, err := h.svc.Get(c.Request.Context(), uid, id)
	if err != nil {
		h.noteErr(c, err)
		return
	}
	core.OK(c, note)
}

func (h *Handler) Update(c *gin.Context) {
	uid, _ := h.userID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 2005, "invalid note id")
		return
	}
	var in UpdateNoteInput
	if err := c.ShouldBindJSON(&in); err != nil {
		core.Fail(c, http.StatusBadRequest, 2005, "invalid body: "+err.Error())
		return
	}
	note, err := h.svc.Update(c.Request.Context(), uid, id, in)
	if err != nil {
		h.noteErr(c, err)
		return
	}
	h.publish(c, note.ID.String(), "note.updated")
	core.OK(c, note)
}

func (h *Handler) Delete(c *gin.Context) {
	uid, _ := h.userID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 2005, "invalid note id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), uid, id); err != nil {
		h.noteErr(c, err)
		return
	}
	h.media.DeleteURLs(h.svc.AttachmentURLs(c.Request.Context(), uid, id))
	h.publish(c, id.String(), "note.deleted")
	core.OK(c, gin.H{"id": id.String()})
}

func (h *Handler) Archive(c *gin.Context) {
	uid, _ := h.userID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 2005, "invalid note id")
		return
	}
	var in struct {
		Archived bool `json:"archived"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		core.Fail(c, http.StatusBadRequest, 2005, "invalid body")
		return
	}
	if err := h.svc.SetArchived(c.Request.Context(), uid, id, in.Archived); err != nil {
		h.noteErr(c, err)
		return
	}
	typ := "note.archived"
	if !in.Archived {
		typ = "note.unarchived"
	}
	h.publish(c, id.String(), typ)
	core.OK(c, gin.H{"id": id.String(), "archived": in.Archived})
}

func (h *Handler) Tags(c *gin.Context) {
	uid, _ := h.userID(c)
	rows, err := h.svc.Tags(c.Request.Context(), uid)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 2007, err.Error())
		return
	}
	core.OK(c, rows)
}

func (h *Handler) Upload(c *gin.Context) {
	uid, _ := h.userID(c)
	file, err := c.FormFile("file")
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 2001, "missing file")
		return
	}
	fh, err := file.Open()
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 2008, err.Error())
		return
	}
	defer fh.Close()
	mime := file.Header.Get("Content-Type")
	url, thumb, size, err := h.media.Save(mime, fh)
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 2001, err.Error())
		return
	}
	var noteID *uuid.UUID
	if v := c.PostForm("noteId"); v != "" {
		if id, perr := uuid.Parse(v); perr == nil {
			noteID = &id
		}
	}
	rec := &model.Attachment{
		UserID:    uid,
		NoteID:    noteID,
		Filename:  file.Filename,
		URL:       url,
		ThumbURL:  thumb,
		SizeBytes: size,
		MimeType:  mime,
	}
	if err := h.svc.SaveAttachment(c.Request.Context(), rec); err != nil {
		core.Fail(c, http.StatusInternalServerError, 2008, err.Error())
		return
	}
	core.OK(c, gin.H{"url": url, "thumbUrl": thumb})
}

func (h *Handler) ExportAll(c *gin.Context) {
	uid, _ := h.userID(c)
	rows, err := h.svc.ExportAll(c.Request.Context(), uid)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 2009, err.Error())
		return
	}
	c.Header("Content-Disposition", "attachment; filename=notes-export.json")
	core.OK(c, rows)
}

func (h *Handler) ExportMarkdown(c *gin.Context) {
	uid, _ := h.userID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 2005, "invalid note id")
		return
	}
	md, err := h.svc.ExportMarkdown(c.Request.Context(), uid, id)
	if err != nil {
		h.noteErr(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=note.md")
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(md))
}

// importMaxBytes 是导入接口的请求体上限，显著高于常规 JSON 接口（笔记正文 + 附带的富文本）。
// 它与全局 MAX_BODY_BYTES 是两笔配额：后者保护常规接口，前者在 handler 内显式抬高。
const importMaxBytes = 10 << 20

func (h *Handler) Import(c *gin.Context) {
	uid, _ := h.userID(c)
	// 导入文件允许比常规 JSON 接口大，因此显式抬高请求体配额；
	// 抬高失败说明链路上没有 BodyLimit，继续即可——此时由下面按实际读到的长度判定。
	middleware.RaiseBodyLimit(c, importMaxBytes)
	// 不再用 io.LimitReader 静默截断：超过上限会读取失败，必须明确告诉调用方，
	// 否则用户导入 20MB 的笔记会「成功导入一半」而毫不知情。
	body, err := io.ReadAll(c.Request.Body)
	if middleware.IsBodyTooLarge(err) {
		core.Fail(c, http.StatusRequestEntityTooLarge, 2010, "导入内容超过上限 "+middleware.HumanBytes(importMaxBytes))
		return
	}
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 2010, "read body failed")
		return
	}
	ct := c.ContentType()
	var notes []ImportedNote
	if strings.Contains(ct, "markdown") {
		notes, err = parseMarkdownImport(body)
	} else {
		notes, err = parseJSONImport(body)
	}
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 2010, err.Error())
		return
	}
	res, err := h.svc.Import(c.Request.Context(), uid, notes)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 2011, err.Error())
		return
	}
	core.OK(c, res)
}

func (h *Handler) Events(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	sessionID := uuid.NewString()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		core.Fail(c, http.StatusInternalServerError, 2009, "当前环境不支持流式响应")
		return
	}
	c.Status(http.StatusOK)
	flusher.Flush()

	sub, stop := h.broker.Register(uid.String(), sessionID)
	defer stop()

	var sinceSeq int64
	if v := c.Query("since"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			sinceSeq = n
		}
	}
	for _, ev := range h.broker.Replay(uid.String(), sinceSeq) {
		writeSSE(c, "replay", ev)
	}
	writeSSE(c, "session", sse.Event{SourceSess: sessionID, EventID: sessionID})
	flusher.Flush()

	ticker := sse.NewHeartbeatTicker()
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case ev := <-sub:
			writeSSE(c, ev.Type, ev)
			flusher.Flush()
		case <-ticker.C:
			c.Writer.WriteString(": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func writeSSE(c *gin.Context, event string, ev sse.Event) {
	payload := ev.String()
	if event == "" {
		event = "note"
	}
	c.Writer.WriteString("event: " + event + "\ndata: " + payload + "\nid: " + ev.EventID + "\n\n")
}

func (h *Handler) publish(c *gin.Context, noteID, typ string) {
	uid, ok := h.userID(c)
	if !ok {
		return
	}
	src := c.GetHeader("X-Session-Id")
	_ = h.broker.Publish(uid.String(), sse.Event{
		NoteID: noteID, Type: typ, SourceSess: src,
	})
}

func (h *Handler) noteErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNoteNotFound):
		core.Fail(c, http.StatusNotFound, 2012, "note not found")
	default:
		core.Fail(c, http.StatusInternalServerError, 2013, err.Error())
	}
}

func parseJSONImport(raw []byte) ([]ImportedNote, error) {
	var arr []ImportedNote
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var one ImportedNote
	if err := json.Unmarshal(raw, &one); err == nil {
		return []ImportedNote{one}, nil
	}
	return nil, errors.New("expected JSON array or object of notes")
}

func parseMarkdownImport(raw []byte) ([]ImportedNote, error) {
	txt := string(raw)
	var tags []string
	title, body := "", txt
	if fm := frontmatter(txt); fm != "" {
		body = txt[len(fm):]
		for k, v := range parseFrontmatter(fm) {
			if k == "title" {
				title = v
			}
			if k == "tags" {
				tags = splitTags(v)
			}
		}
	}
	if title == "" {
		title = "Imported note"
	}
	return []ImportedNote{{Title: title, Body: body, Tags: tags}}, nil
}

func frontmatter(s string) string {
	if !strings.HasPrefix(s, "---") {
		return ""
	}
	end := strings.Index(s[3:], "---")
	if end < 0 {
		return ""
	}
	return s[:end+3]
}

func parseFrontmatter(fm string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(strings.Trim(fm, "-\n"), "\n")
	for _, l := range lines {
		parts := strings.SplitN(l, ":", 2)
		if len(parts) != 2 {
			continue
		}
		out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return out
}

func splitTags(v string) []string {
	v = strings.Trim(v, "[] ")
	if v == "" {
		return nil
	}
	items := strings.Split(v, ",")
	out := make([]string, 0, len(items))
	for _, i := range items {
		i = strings.TrimSpace(i)
		if i != "" {
			out = append(out, i)
		}
	}
	return out
}

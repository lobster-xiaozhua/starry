package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"login-system/backend/internal/model"
	"login-system/backend/internal/service"
	"login-system/backend/internal/sse"
	"login-system/backend/internal/store"
)

type NotesHandler struct {
	svc    *service.NotesService
	media  *store.FileStore
	broker *sse.Broker
}

func NewNotesHandler(svc *service.NotesService, media *store.FileStore, broker *sse.Broker) *NotesHandler {
	return &NotesHandler{svc: svc, media: media, broker: broker}
}

func (h *NotesHandler) userID(c *gin.Context) (uuid.UUID, bool) {
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

func (h *NotesHandler) List(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	size, _ := strconv.Atoi(c.Query("size"))
	if c.Query("q") != "" {
		notes, total, err := h.svc.Search(c.Request.Context(), uid, c.Query("q"), page, size)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 2004, err.Error())
			return
		}
		OK(c, gin.H{"notes": notes, "total": total, "page": page, "size": size})
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
	notes, total, err := h.svc.List(c.Request.Context(), store.NoteListQuery{
		UserID: uid, Tags: tags, Archived: arch, Page: page, Size: size,
	})
	if err != nil {
		Fail(c, http.StatusInternalServerError, 2004, err.Error())
		return
	}
	OK(c, gin.H{"notes": notes, "total": total, "page": page, "size": size})
}

// SeedDemo POST /api/notes/seed-demo
// 为该用户灌入示例笔记（仅在尚无笔记时执行，幂等）。便于新用户一键体验产品。
func (h *NotesHandler) SeedDemo(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 2003, "invalid user")
		return
	}
	n, err := h.svc.SeedDemo(c.Request.Context(), uid)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 2004, err.Error())
		return
	}
	if n == 0 {
		OK(c, gin.H{"count": 0, "message": "已有笔记，未重复灌入"})
		return
	}
	h.publish(c, "", "note.created")
	OK(c, gin.H{"count": n, "message": "已为你加载示例笔记"})
}

func (h *NotesHandler) Create(c *gin.Context) {
	uid, _ := h.userID(c)
	var in service.CreateNoteInput
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, 2005, "invalid body: "+err.Error())
		return
	}
	note, err := h.svc.Create(c.Request.Context(), uid, in)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 2006, err.Error())
		return
	}
	h.publish(c, note.ID.String(), "note.created")
	OK(c, note)
}

func (h *NotesHandler) Get(c *gin.Context) {
	uid, _ := h.userID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, http.StatusBadRequest, 2005, "invalid note id")
		return
	}
	note, err := h.svc.Get(c.Request.Context(), uid, id)
	if err != nil {
		h.noteErr(c, err)
		return
	}
	OK(c, note)
}

func (h *NotesHandler) Update(c *gin.Context) {
	uid, _ := h.userID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, http.StatusBadRequest, 2005, "invalid note id")
		return
	}
	var in service.UpdateNoteInput
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, 2005, "invalid body: "+err.Error())
		return
	}
	note, err := h.svc.Update(c.Request.Context(), uid, id, in)
	if err != nil {
		h.noteErr(c, err)
		return
	}
	h.publish(c, note.ID.String(), "note.updated")
	OK(c, note)
}

func (h *NotesHandler) Delete(c *gin.Context) {
	uid, _ := h.userID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, http.StatusBadRequest, 2005, "invalid note id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), uid, id); err != nil {
		h.noteErr(c, err)
		return
	}
	h.media.DeleteURLs(h.svc.AttachmentURLs(c.Request.Context(), uid, id))
	h.publish(c, id.String(), "note.deleted")
	OK(c, gin.H{"id": id.String()})
}

func (h *NotesHandler) Archive(c *gin.Context) {
	uid, _ := h.userID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, http.StatusBadRequest, 2005, "invalid note id")
		return
	}
	var in struct {
		Archived bool `json:"archived"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, 2005, "invalid body")
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
	OK(c, gin.H{"id": id.String(), "archived": in.Archived})
}

func (h *NotesHandler) Tags(c *gin.Context) {
	uid, _ := h.userID(c)
	rows, err := h.svc.Tags(c.Request.Context(), uid)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 2007, err.Error())
		return
	}
	OK(c, rows)
}

func (h *NotesHandler) Upload(c *gin.Context) {
	uid, _ := h.userID(c)
	file, err := c.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, 2001, "missing file")
		return
	}
	fh, err := file.Open()
	if err != nil {
		Fail(c, http.StatusInternalServerError, 2008, err.Error())
		return
	}
	defer fh.Close()
	mime := file.Header.Get("Content-Type")
	url, thumb, size, err := h.media.Save(mime, fh)
	if err != nil {
		Fail(c, http.StatusBadRequest, 2001, err.Error())
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
		Fail(c, http.StatusInternalServerError, 2008, err.Error())
		return
	}
	OK(c, gin.H{"url": url, "thumbUrl": thumb})
}

func (h *NotesHandler) ExportAll(c *gin.Context) {
	uid, _ := h.userID(c)
	rows, err := h.svc.ExportAll(c.Request.Context(), uid)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 2009, err.Error())
		return
	}
	c.Header("Content-Disposition", "attachment; filename=notes-export.json")
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": rows})
}

func (h *NotesHandler) ExportMarkdown(c *gin.Context) {
	uid, _ := h.userID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, http.StatusBadRequest, 2005, "invalid note id")
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

func (h *NotesHandler) Import(c *gin.Context) {
	uid, _ := h.userID(c)
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 10<<20))
	if err != nil {
		Fail(c, http.StatusBadRequest, 2010, "read body failed")
		return
	}
	ct := c.ContentType()
	var notes []service.ImportedNote
	if strings.Contains(ct, "markdown") {
		notes, err = parseMarkdownImport(body)
	} else {
		notes, err = parseJSONImport(body)
	}
	if err != nil {
		Fail(c, http.StatusBadRequest, 2010, err.Error())
		return
	}
	res, err := h.svc.Import(c.Request.Context(), uid, notes)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 2011, err.Error())
		return
	}
	OK(c, res)
}

func (h *NotesHandler) Events(c *gin.Context) {
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
		c.AbortWithStatus(http.StatusInternalServerError)
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

func (h *NotesHandler) publish(c *gin.Context, noteID, typ string) {
	uid, ok := h.userID(c)
	if !ok {
		return
	}
	src := c.GetHeader("X-Session-Id")
	_ = h.broker.Publish(uid.String(), sse.Event{
		NoteID: noteID, Type: typ, SourceSess: src,
	})
}

func (h *NotesHandler) noteErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNoteNotFound):
		Fail(c, http.StatusNotFound, 2012, "note not found")
	default:
		Fail(c, http.StatusInternalServerError, 2013, err.Error())
	}
}

func parseJSONImport(raw []byte) ([]service.ImportedNote, error) {
	var arr []service.ImportedNote
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var one service.ImportedNote
	if err := json.Unmarshal(raw, &one); err == nil {
		return []service.ImportedNote{one}, nil
	}
	return nil, errors.New("expected JSON array or object of notes")
}

func parseMarkdownImport(raw []byte) ([]service.ImportedNote, error) {
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
	return []service.ImportedNote{{Title: title, Body: body, Tags: tags}}, nil
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

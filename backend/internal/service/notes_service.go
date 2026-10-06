package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"login-system/backend/internal/model"
	"login-system/backend/internal/store"
)

type NotesService struct {
	db *store.DB
}

func NewNotesService(db *store.DB) *NotesService {
	return &NotesService{db: db}
}

var (
	ErrNoteNotFound = errors.New("note not found")
)

type CreateNoteInput struct {
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
}

type UpdateNoteInput struct {
	Title *string  `json:"title"`
	Body  *string  `json:"body"`
	Tags  []string `json:"tags"`
}

func (s *NotesService) Create(ctx context.Context, userID uuid.UUID, in CreateNoteInput) (*model.Note, error) {
	title := in.Title
	if len([]rune(title)) == 0 {
		title = "未命名笔记"
	}
	n := &model.Note{UserID: userID, Title: title, Body: in.Body}
	if err := s.db.CreateNote(n); err != nil {
		return nil, err
	}
	if err := s.db.LinkNoteTags(n.ID, userID, in.Tags); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *NotesService) Get(ctx context.Context, userID, noteID uuid.UUID) (*model.NoteWithTags, error) {
	n, err := s.db.GetNote(userID, noteID)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return nil, ErrNoteNotFound
	}
	return n, nil
}

func (s *NotesService) Update(ctx context.Context, userID, noteID uuid.UUID, in UpdateNoteInput) (*model.Note, error) {
	existing, err := s.db.GetNote(userID, noteID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrNoteNotFound
	}
	n := existing.Note
	if in.Title != nil {
		n.Title = *in.Title
	}
	if in.Body != nil {
		n.Body = *in.Body
	}
	if err := s.db.UpdateNote(&n); err != nil {
		return nil, err
	}
	if in.Tags != nil {
		if err := s.db.LinkNoteTags(noteID, userID, in.Tags); err != nil {
			return nil, err
		}
	}
	return &n, nil
}

func (s *NotesService) SetArchived(ctx context.Context, userID, noteID uuid.UUID, archived bool) error {
	if _, err := s.db.GetNote(userID, noteID); err != nil {
		return err
	}
	return s.db.SetNoteArchived(userID, noteID, archived)
}

func (s *NotesService) Delete(ctx context.Context, userID, noteID uuid.UUID) error {
	existing, err := s.db.GetNote(userID, noteID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrNoteNotFound
	}
	return s.db.DeleteNote(userID, noteID)
}

func (s *NotesService) List(ctx context.Context, q store.NoteListQuery) ([]model.NoteWithTags, int64, error) {
	return s.db.ListNotes(q)
}

// SeedDemo 为该用户灌入示例笔记（仅在尚无笔记时执行，幂等），返回创建条数。
func (s *NotesService) SeedDemo(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.db.SeedDemoNotes(userID)
}

func (s *NotesService) Tags(ctx context.Context, userID uuid.UUID) ([]model.TagCount, error) {
	rows, err := s.db.ListTags(userID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.TagCount{}
	}
	return rows, nil
}

func (s *NotesService) Search(ctx context.Context, userID uuid.UUID, query string, page, size int) ([]model.NoteWithTags, int64, error) {
	if query == "" {
		q := store.NoteListQuery{UserID: userID, Page: page, Size: size}
		return s.List(ctx, q)
	}
	out, total, err := s.db.SearchNotes(userID, query, page, size)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

type ImportedNote struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
}

type ImportResult struct {
	OK      int      `json:"ok"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}

func (s *NotesService) Import(ctx context.Context, userID uuid.UUID, notes []ImportedNote) (*ImportResult, error) {
	res := &ImportResult{}
	for i, n := range notes {
		if n.Title == "" {
			res.Errors = append(res.Errors, "row#"+itoa(i+1)+": missing title")
			continue
		}
		if n.ID != "" {
			exists, err := s.db.NoteExistsByStableID(userID, n.ID)
			if err != nil {
				res.Errors = append(res.Errors, "row#"+itoa(i+1)+": "+err.Error())
				continue
			}
			if exists {
				res.Skipped++
				continue
			}
		}
		created, err := s.Create(ctx, userID, CreateNoteInput{Title: n.Title, Body: n.Body, Tags: n.Tags})
		if err != nil {
			res.Errors = append(res.Errors, "row#"+itoa(i+1)+": "+err.Error())
			continue
		}
		if n.ID != "" {
			if err := s.db.SaveNoteStableID(created.ID, userID, n.ID); err != nil {
				res.Errors = append(res.Errors, "row#"+itoa(i+1)+": stable id save failed")
				continue
			}
		}
		res.OK++
	}
	return res, nil
}

// AttachmentURLs 返回指定笔记所有附件的磁盘清理 URL 列表。
func (s *NotesService) AttachmentURLs(ctx context.Context, userID, noteID uuid.UUID) []string {
	rows, err := s.db.ListAttachmentsByNote(userID, noteID)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, a := range rows {
		out = append(out, a.URL)
		if a.ThumbURL != "" {
			out = append(out, a.ThumbURL)
		}
	}
	return out
}

// SaveAttachment 落库附件记录。
func (s *NotesService) SaveAttachment(ctx context.Context, a *model.Attachment) error {
	return s.db.SaveAttachment(a)
}

// ExportAll 导出全量笔记（含稳定 ID 与标签）。分页累计，避免单次 Size 上限
// 静默截断导致笔记数超过上限时导出丢数据。
func (s *NotesService) ExportAll(ctx context.Context, userID uuid.UUID) ([]ExportRow, error) {
	const pageSize = 200
	collect := func(archived bool) ([]model.NoteWithTags, error) {
		out := make([]model.NoteWithTags, 0)
		for page := 1; ; page++ {
			arc := archived
			notes, total, err := s.List(ctx, store.NoteListQuery{
				UserID:   userID,
				Archived: &arc,
				Page:     page,
				Size:     pageSize,
			})
			if err != nil {
				return nil, err
			}
			out = append(out, notes...)
			if int64(len(out)) >= total || len(notes) == 0 {
				break
			}
		}
		return out, nil
	}
	active, err := collect(false)
	if err != nil {
		return nil, err
	}
	archived, err := collect(true)
	if err != nil {
		return nil, err
	}
	notes := append(active, archived...)
	out := make([]ExportRow, 0, len(notes))
	for _, n := range notes {
		out = append(out, ExportRow{
			ID:       n.ID.String(),
			Title:   n.Title,
			Body:    n.Body,
			Archived: n.Archived,
			Tags:    n.Tags,
		})
	}
	return out, nil
}

type ExportRow struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Archived bool     `json:"archived"`
	Tags     []string `json:"tags"`
}

// ExportMarkdown 导出单条笔记（frontmatter 含标签）。
func (s *NotesService) ExportMarkdown(ctx context.Context, userID, noteID uuid.UUID) (string, error) {
	n, err := s.Get(ctx, userID, noteID)
	if err != nil {
		return "", err
	}
	return toMarkdown(n), nil
}

func toMarkdown(n *model.NoteWithTags) string {
	tags := ""
	if len(n.Tags) > 0 {
		tags = n.Tags[0]
		for i := 1; i < len(n.Tags); i++ {
			tags += ", " + n.Tags[i]
		}
	}
	return "---\ntitle: " + n.Title + "\ntags: [" + tags + "]\narchived: " + boolStr(n.Archived) + "\n---\n\n" + n.Body
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10%10)) + string(rune('0'+i%10))
}

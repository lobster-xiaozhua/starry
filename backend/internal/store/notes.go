package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"starry/backend/internal/model"
)

type NoteListQuery struct {
	UserID   uuid.UUID
	Tags     []string
	Archived *bool
	Page     int
	Size     int
	Sort     string // 已由 request.Sort 白名单化，安全可直拼 ORDER BY；空则回落默认
}

func (s *DB) CreateNote(n *model.Note) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	now := time.Now()
	n.CreatedAt = now
	n.UpdatedAt = now
	return s.gorm.Create(n).Error
}

func (s *DB) GetNote(userID, noteID uuid.UUID) (*model.NoteWithTags, error) {
	var n model.Note
	err := s.gorm.Where("id = ? AND user_id = ?", noteID, userID).First(&n).Error
	if err != nil {
		if errors2(err) {
			return nil, nil
		}
		return nil, err
	}
	out := model.NoteWithTags{Note: n}
	out.Tags, _ = s.NoteTags(n.ID)
	return &out, nil
}

func (s *DB) UpdateNote(n *model.Note) error {
	now := time.Now()
	return s.gorm.Model(&model.Note{}).
		Where("id = ? AND user_id = ?", n.ID, n.UserID).
		Updates(map[string]interface{}{
			"title":      n.Title,
			"body":       n.Body,
			"archived":   n.Archived,
			"updated_at": now,
		}).Error
}

func (s *DB) SetNoteArchived(userID, noteID uuid.UUID, archived bool) error {
	return s.gorm.Model(&model.Note{}).
		Where("id = ? AND user_id = ?", noteID, userID).
		Updates(map[string]interface{}{"archived": archived, "updated_at": time.Now()}).Error
}

func (s *DB) DeleteNote(userID, noteID uuid.UUID) error {
	res := s.gorm.Unscoped().Where("id = ? AND user_id = ?", noteID, userID).
		Delete(&model.Note{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nil
	}
	return s.DeleteAttachmentsByNote(noteID)
}

func (s *DB) ListNotes(q NoteListQuery) ([]model.NoteWithTags, int64, error) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.Size
	if size < 1 || size > 100 {
		size = 20
	}

	base := s.gorm.Model(&model.Note{}).Where("user_id = ?", q.UserID)
	if q.Archived != nil {
		base = base.Where("archived = ?", *q.Archived)
	}
	if len(q.Tags) > 0 {
		placeholders := make([]string, len(q.Tags))
		for i := range q.Tags {
			placeholders[i] = fmt.Sprintf("?")
		}
		subSQL := "SELECT 1 FROM note_tags WHERE note_tags.note_id = notes.id " +
			"AND note_tags.tag_id IN (SELECT t.id FROM tags t WHERE t.user_id = ? AND t.name IN (" +
			strings.Join(placeholders, ",") + ")) GROUP BY note_tags.note_id " +
			fmt.Sprintf("HAVING COUNT(DISTINCT note_tags.tag_id) = %d", len(q.Tags))
		args := []interface{}{q.UserID}
		for _, t := range q.Tags {
			args = append(args, t)
		}
		base = base.Where("EXISTS ("+subSQL+")", args...)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var notes []model.Note
	orderBy := q.Sort
	if orderBy == "" {
		orderBy = "updated_at DESC"
	}
	err := base.Order(orderBy).
		Offset((page - 1) * size).Limit(size).
		Find(&notes).Error
	if err != nil {
		return nil, 0, err
	}

	out := make([]model.NoteWithTags, 0, len(notes))
	ids := make([]uuid.UUID, 0, len(notes))
	for i := range notes {
		ids = append(ids, notes[i].ID)
	}
	tagMap, err := s.TagsByNoteIDs(q.UserID, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range notes {
		// 无标签的笔记在 tagMap 中无对应键，取零值为 nil；
		// 统一返回空切片，避免前端按 null 处理时访问 .length 崩溃（白屏）。
		tags := tagMap[notes[i].ID.String()]
		if tags == nil {
			tags = []string{}
		}
		out = append(out, model.NoteWithTags{
			Note: notes[i],
			Tags: tags,
		})
	}
	return out, total, nil
}

func (s *DB) UpsertTags(userID uuid.UUID, names []string) ([]model.Tag, error) {
	clean := normalizeTags(names)
	if len(clean) == 0 {
		return []model.Tag{}, nil
	}
	var tags []model.Tag
	for _, name := range clean {
		var t model.Tag
		err := s.gorm.Where("user_id = ? AND lower(name) = ?", userID, strings.ToLower(name)).
			First(&t).Error
		if err == gorm.ErrRecordNotFound {
			t = model.Tag{ID: uuid.New(), UserID: userID, Name: name}
			if cerr := s.gorm.Create(&t).Error; cerr != nil {
				return nil, cerr
			}
		} else if err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *DB) LinkNoteTags(noteID, userID uuid.UUID, names []string) error {
	s.gorm.Where("note_id = ?", noteID).Delete(&model.NoteTag{})
	tags, err := s.UpsertTags(userID, names)
	if err != nil {
		return err
	}
	links := make([]model.NoteTag, 0, len(tags))
	for _, t := range tags {
		links = append(links, model.NoteTag{NoteID: noteID, TagID: t.ID})
	}
	if len(links) > 0 {
		return s.gorm.Create(&links).Error
	}
	return nil
}

func (s *DB) ListTags(userID uuid.UUID) ([]model.TagCount, error) {
	var rows []model.TagCount
	err := s.gorm.
		Model(&model.NoteTag{}).
		Joins("JOIN tags t ON t.id = note_tags.tag_id").
		Where("t.user_id = ?", userID).
		Group("t.name").
		Select("t.name AS name, COUNT(*) AS count").
		Order("count DESC").
		Scan(&rows).Error
	return rows, err
}

func (s *DB) NoteTags(noteID uuid.UUID) ([]string, error) {
	var rows []model.Tag
	err := s.gorm.Model(&model.NoteTag{}).
		Joins("JOIN tags t ON t.id = note_tags.tag_id").
		Where("note_tags.note_id = ?", noteID).
		Select("t.name").
		Scan(&rows).Error
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.Name)
	}
	return names, err
}

func (s *DB) TagsByNoteIDs(userID uuid.UUID, noteIDs []uuid.UUID) (map[string][]string, error) {
	result := map[string][]string{}
	if len(noteIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		NoteID uuid.UUID
		Name   string
	}
	err := s.gorm.Model(&model.NoteTag{}).
		Joins("JOIN tags t ON t.id = note_tags.tag_id").
		Where("note_tags.note_id IN ? AND t.user_id = ?", noteIDs, userID).
		Select("note_tags.note_id, t.name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.NoteID.String()] = append(result[r.NoteID.String()], r.Name)
	}
	for k := range result {
		if len(result[k]) == 0 {
			result[k] = []string{}
		}
	}
	return result, nil
}

func (s *DB) ListAttachmentsByNote(userID, noteID uuid.UUID) ([]model.Attachment, error) {
	var rows []model.Attachment
	err := s.gorm.Where("user_id = ? AND note_id = ?", userID, noteID).
		Order("created_at ASC").Find(&rows).Error
	return rows, err
}

func (s *DB) SaveAttachment(a *model.Attachment) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	a.CreatedAt = time.Now()
	return s.gorm.Create(a).Error
}

func (s *DB) DeleteAttachmentsByNote(noteID uuid.UUID) error {
	return s.gorm.Where("note_id = ?", noteID).Delete(&model.Attachment{}).Error
}

func (s *DB) SaveNoteStableID(noteID, userID uuid.UUID, stable string) error {
	row := model.NoteStableID{NoteID: noteID, UserID: userID, Stable: stable}
	return s.gorm.Create(&row).Error
}

func (s *DB) NoteExistsByStableID(userID uuid.UUID, stable string) (bool, error) {
	var count int64
	err := s.gorm.Model(&model.NoteStableID{}).
		Where("user_id = ? AND stable = ?", userID, stable).
		Count(&count).Error
	return count > 0, err
}

func errors2(err error) bool {
	return err == gorm.ErrRecordNotFound
}

func normalizeTags(names []string) []string {
	seen := map[string]string{}
	out := []string{}
	for _, n := range names {
		trimmed := strings.TrimSpace(n)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = trimmed
		out = append(out, trimmed)
	}
	return out
}

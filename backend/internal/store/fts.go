package store

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"starry/backend/internal/model"
)

const shortQueryLen = 4

func (s *DB) SearchNotes(userID uuid.UUID, query string, page, size int) ([]model.NoteWithTags, int64, error) {
	page = normalizePage(page)
	size = normalizeSize(size)
	if len([]rune(query)) < shortQueryLen {
		return s.searchShort(userID, query, page, size)
	}
	return s.searchFTS(userID, query, page, size)
}

func (s *DB) searchShort(userID uuid.UUID, q string, page, size int) ([]model.NoteWithTags, int64, error) {
	like := "%" + q + "%"
	base := s.gorm.Model(&model.Note{}).
		Where("user_id = ?", userID).
		Where("title ILIKE ? OR body ILIKE ? OR EXISTS (SELECT 1 FROM note_tags nt JOIN tags t ON t.id = nt.tag_id WHERE nt.note_id = notes.id AND t.user_id = ? AND t.name ILIKE ?)", like, like, userID, like)
	total, notes, err := s.pageQuery(base, page, size)
	if err != nil {
		return nil, 0, err
	}
	return s.attachTags(userID, notes), total, nil
}

func (s *DB) searchFTS(userID uuid.UUID, q string, page, size int) ([]model.NoteWithTags, int64, error) {
	base := s.gorm.Model(&model.Note{}).
		Where("user_id = ?", userID).
		Where("fts @@ websearch_to_tsquery('simple', ?)", q)
	total, notes, err := s.pageQuery(base, page, size)
	if err != nil {
		return nil, 0, err
	}
	return s.attachTags(userID, notes), total, nil
}

func (s *DB) pageQuery(q *gorm.DB, page, size int) (int64, []model.Note, error) {
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var notes []model.Note
	if err := q.Order("updated_at DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&notes).Error; err != nil {
		return 0, nil, err
	}
	return total, notes, nil
}

func (s *DB) attachTags(userID uuid.UUID, notes []model.Note) []model.NoteWithTags {
	ids := make([]uuid.UUID, 0, len(notes))
	for i := range notes {
		ids = append(ids, notes[i].ID)
	}
	tagMap, _ := s.TagsByNoteIDs(userID, ids)
	out := make([]model.NoteWithTags, 0, len(notes))
	for i := range notes {
		out = append(out, model.NoteWithTags{
			Note: notes[i],
			Tags: tagMap[notes[i].ID.String()],
		})
	}
	return out
}

func normalizePage(p int) int {
	if p < 1 {
		return 1
	}
	return p
}

func normalizeSize(z int) int {
	if z < 1 || z > 100 {
		return 20
	}
	return z
}

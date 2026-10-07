package notes

import (
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立云笔记模块拥有的表，并创建本模块专属的全文检索结构。
//
// fts 生成列与 GIN 索引是笔记搜索（含标题模糊匹配）的性能基础，与笔记表同属本模块，
// 因此集中在此维护，而不是散落在全局迁移里。
func Migrate(db *store.DB) error {
	if err := db.AutoMigrate(
		&model.Note{}, &model.Tag{}, &model.NoteTag{},
		&model.NoteStableID{}, &model.Attachment{},
	); err != nil {
		return err
	}
	return db.ExecDDL(
		"ALTER TABLE notes ADD COLUMN IF NOT EXISTS fts tsvector GENERATED ALWAYS AS "+
			"(setweight(to_tsvector('simple', coalesce(title, '')), 'A') || "+
			"setweight(to_tsvector('simple', coalesce(body, '')), 'B')) STORED",
		"CREATE INDEX IF NOT EXISTS idx_notes_user ON notes(user_id, archived, updated_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_notes_fts ON notes USING GIN (fts)",
		"CREATE INDEX IF NOT EXISTS idx_notes_title_trgm ON notes USING GIN (title gin_trgm_ops)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_user_name ON tags(user_id, name)",
	)
}

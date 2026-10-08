package notes

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

func newTestService(t *testing.T) (*NotesService, *store.DB) {
	t.Helper()
	g, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := g.AutoMigrate(&model.Note{}, &model.Tag{}, &model.NoteTag{}, &model.NoteStableID{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db := store.NewFromGorm(g)
	return NewService(db), db
}

func countNotes(t *testing.T, db *store.DB, uid uuid.UUID) int64 {
	t.Helper()
	var c int64
	if err := db.Gorm().Model(&model.Note{}).Where("user_id = ?", uid).Count(&c).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	return c
}

// TestImportNoOrphanWhenTagLinkFails 是「导入不产生孤儿笔记」的回归测试。
//
// 复现手法：直接 DROP note_tags 表，使 LinkNoteTags 必然失败——这正是修复前
// 「笔记行已写入、但标签关联失败」的孤儿场景。修复后整行处于一个事务内，
// 标签关联失败应整体回滚，笔记不得落库；而修复前的实现会留下 count=1 的孤儿。
func TestImportNoOrphanWhenTagLinkFails(t *testing.T) {
	svc, db := newTestService(t)
	uid := uuid.New()
	if err := db.Gorm().Exec("DROP TABLE note_tags").Error; err != nil {
		t.Fatalf("drop note_tags: %v", err)
	}

	res, err := svc.Import(context.Background(), uid, []ImportedNote{
		{Title: "will-fail-tags", Tags: []string{"x"}},
	})
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if res.OK != 0 {
		t.Fatalf("expected 0 ok, got %d", res.OK)
	}
	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(res.Errors), res.Errors)
	}
	if got := countNotes(t, db, uid); got != 0 {
		t.Fatalf("orphan note leaked after rollback: count=%d, want 0", got)
	}
}

// TestImportHappyPathAndSkip 覆盖导入的正常路径与稳定 ID 去重：
// 合法行全部创建、缺标题行记入 Errors、重复稳定 ID 计入 Skipped 且不重复落库。
func TestImportHappyPathAndSkip(t *testing.T) {
	svc, db := newTestService(t)
	uid := uuid.New()

	res, err := svc.Import(context.Background(), uid, []ImportedNote{
		{ID: "s1", Title: "a"},
		{ID: "s2", Title: "b", Tags: []string{"go", "db"}},
		{Title: ""}, // 缺标题 -> error
	})
	if err != nil {
		t.Fatalf("Import error: %v", err)
	}
	if res.OK != 2 {
		t.Fatalf("expected 2 ok, got %d", res.OK)
	}
	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 error (missing title), got %d: %v", len(res.Errors), res.Errors)
	}
	if got := countNotes(t, db, uid); got != 2 {
		t.Fatalf("expected 2 notes, got %d", got)
	}

	// 导入的笔记应可取回，且标签关联成功。
	notes, _, err := svc.List(context.Background(), store.NoteListQuery{UserID: uid, Page: 1, Size: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 listed notes, got %d", len(notes))
	}
	found := false
	for _, n := range notes {
		if n.Title == "b" {
			found = true
			if len(n.Tags) != 2 {
				t.Fatalf("expected 2 tags on note b, got %d (%v)", len(n.Tags), n.Tags)
			}
		}
	}
	if !found {
		t.Fatalf("note b not found in list")
	}

	// 重复稳定 ID 应被跳过，不重复创建。
	res2, err := svc.Import(context.Background(), uid, []ImportedNote{
		{ID: "s1", Title: "a-again"},
		{ID: "s3", Title: "c"},
	})
	if err != nil {
		t.Fatalf("Import2 error: %v", err)
	}
	if res2.OK != 1 || res2.Skipped != 1 {
		t.Fatalf("expected ok=1 skipped=1, got ok=%d skipped=%d", res2.OK, res2.Skipped)
	}
	if got := countNotes(t, db, uid); got != 3 {
		t.Fatalf("expected 3 notes total, got %d", got)
	}
}

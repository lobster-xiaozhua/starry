package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"starry/backend/internal/model"
)

// openIntegrityDB 建一个带全部业务表的内存外 sqlite 库（独立临时文件，测试间隔离），
// 并挂载真实的临时磁盘目录（网盘 + 附件），供销户级联清理测试使用。
func openIntegrityDB(t *testing.T) *DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "integrity.db")
	g, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := g.AutoMigrate(
		&model.User{}, &model.Note{}, &model.Tag{}, &model.NoteTag{}, &model.NoteStableID{},
		&model.Attachment{}, &model.Board{}, &model.BoardColumn{}, &model.BoardTask{},
		&model.KnowledgeDoc{}, &model.KnowledgeChunk{}, &model.VaultKey{}, &model.VaultItem{},
		&model.Conversation{}, &model.Message{}, &model.AgentTask{},
		&model.DriveFile{}, &model.DriveQuota{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db := NewFromGorm(g)
	db.Drive = NewDriveStore(t.TempDir())
	mediaDir := t.TempDir()
	// 附件缩略图目录由 Init 创建；这里手工建好以便落真实文件。
	if err := os.MkdirAll(filepath.Join(mediaDir, "thumbs"), 0o755); err != nil {
		t.Fatalf("mkdir thumbs: %v", err)
	}
	db.Media = NewFileStore(mediaDir)
	return db
}

func mustCreate(t *testing.T, db *DB, v any) {
	t.Helper()
	if err := db.gorm.Create(v).Error; err != nil {
		t.Fatalf("seed %T: %v", v, err)
	}
}

func countRows(t *testing.T, db *DB, m any, query string, args ...any) int64 {
	t.Helper()
	var c int64
	if err := db.gorm.Unscoped().Model(m).Where(query, args...).Count(&c).Error; err != nil {
		t.Fatalf("count %T: %v", m, err)
	}
	return c
}

// TestPurgeUserData 是安全销户的精确回归：种入用户在各模块的数据（含真实落盘的
// 网盘文件与附件、以及一条已被软删除的笔记），执行 PurgeUserData 后断言——
// 所有表行清空、磁盘文件已删、配额行已删；且其他用户的数据不受影响。
func TestPurgeUserData(t *testing.T) {
	db := openIntegrityDB(t)
	ctx := context.Background()
	uid := uuid.New()
	other := uuid.New()

	mustCreate(t, db, &model.User{ID: uid, Username: "victim", Email: "v@x.com", PasswordHash: "x", Role: "user", Status: "active"})
	mustCreate(t, db, &model.User{ID: other, Username: "keeper", Email: "k@x.com", PasswordHash: "x", Role: "user", Status: "active"})

	// 笔记（一条存活、一条已软删除）+ 标签 + 关联 + 稳定 ID
	liveNote := &model.Note{ID: uuid.New(), UserID: uid, Title: "live", Body: "b"}
	deadNote := &model.Note{ID: uuid.New(), UserID: uid, Title: "dead", Body: "b"}
	mustCreate(t, db, liveNote)
	mustCreate(t, db, deadNote)
	if err := db.gorm.Delete(deadNote).Error; err != nil { // 软删除
		t.Fatalf("soft delete: %v", err)
	}
	tag := &model.Tag{ID: uuid.New(), UserID: uid, Name: "work"}
	mustCreate(t, db, tag)
	mustCreate(t, db, &model.NoteTag{NoteID: liveNote.ID, TagID: tag.ID})
	mustCreate(t, db, &model.NoteStableID{NoteID: liveNote.ID, UserID: uid, Stable: "s-1"})
	// 其他用户的笔记：销户后必须仍在
	otherNote := &model.Note{ID: uuid.New(), UserID: other, Title: "keep-me", Body: "b"}
	mustCreate(t, db, otherNote)

	// 附件：真实落盘（原图 + 缩略图）+ 元数据行
	mediaOriginal := db.Media.PathForURL("/uploads/att.png")
	mediaThumb := db.Media.PathForURL("/uploads/thumbs/att.png.jpg")
	if err := os.WriteFile(mediaOriginal, []byte("img"), 0o644); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	if err := os.WriteFile(mediaThumb, []byte("thumb"), 0o644); err != nil {
		t.Fatalf("write thumb: %v", err)
	}
	mustCreate(t, db, &model.Attachment{ID: uuid.New(), UserID: uid, Filename: "att.png",
		URL: "/uploads/att.png", ThumbURL: "/uploads/thumbs/att.png.jpg", MimeType: "image/png"})

	// 网盘：真实落盘文件 + 目录行 + 配额占用
	storedName, err := db.Drive.Save(uid, strings.NewReader("drive-bytes"))
	if err != nil {
		t.Fatalf("drive save: %v", err)
	}
	drivePath := db.Drive.Path(uid, storedName)
	mustCreate(t, db, &model.DriveFile{ID: uuid.New(), UserID: uid, Name: "f.bin",
		Size: 10, StoredName: storedName, IsDir: false})
	mustCreate(t, db, &model.DriveFile{ID: uuid.New(), UserID: uid, Name: "folder", IsDir: true})
	if err := db.EnsureDriveQuotaRow(uid); err != nil {
		t.Fatalf("ensure quota: %v", err)
	}
	if ok, err := db.ClaimDriveSpace(uid, 10, 1000); err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}

	// 看板 / 知识库 / 保险箱 / 对话 / 消息 / 长程任务
	board := &model.Board{ID: uuid.New(), UserID: uid, Name: "b"}
	mustCreate(t, db, board)
	mustCreate(t, db, &model.BoardColumn{ID: uuid.New(), BoardID: board.ID, UserID: uid, Title: "c"})
	mustCreate(t, db, &model.BoardTask{ID: uuid.New(), UserID: uid, BoardID: board.ID, ColumnID: uuid.New(), Title: "t"})
	doc := &model.KnowledgeDoc{ID: uuid.New(), UserID: uid, Title: "d"}
	mustCreate(t, db, doc)
	mustCreate(t, db, &model.KnowledgeChunk{ID: uuid.New(), DocID: doc.ID, UserID: uid, Content: "c"})
	mustCreate(t, db, &model.VaultKey{ID: uuid.New(), UserID: uid, Salt: "s", Verifier: "v"})
	mustCreate(t, db, &model.VaultItem{ID: uuid.New(), UserID: uid, Title: "i", Encrypted: "e"})
	conv := &model.Conversation{ID: uuid.New(), UserID: uid, Title: "cv"}
	mustCreate(t, db, conv)
	mustCreate(t, db, &model.Message{ID: uuid.New(), ConversationID: conv.ID, UserID: uid, Role: "user", Content: "m"})
	mustCreate(t, db, &model.AgentTask{ID: uuid.New(), UserID: uid, Goal: "g", Status: "done"})

	// 磁盘文件与配额行在销户前应存在
	if _, err := os.Stat(drivePath); err != nil {
		t.Fatalf("drive file should exist before purge: %v", err)
	}
	if _, err := os.Stat(mediaOriginal); err != nil {
		t.Fatalf("attachment should exist before purge: %v", err)
	}
	if c := countRows(t, db, &model.DriveQuota{}, "user_id = ?", uid); c != 1 {
		t.Fatalf("quota row before purge = %d, want 1", c)
	}

	if err := db.PurgeUserData(ctx, uid); err != nil {
		t.Fatalf("purge: %v", err)
	}

	// ===== 断言：该用户所有数据清空（按用户范围计数；其他用户数据在下方单独断言）=====
	userScoped := []struct {
		name string
		m    any
	}{
		{"notes", &model.Note{}}, {"tags", &model.Tag{}},
		{"note_stable_ids", &model.NoteStableID{}},
		{"attachments", &model.Attachment{}}, {"boards", &model.Board{}},
		{"board_columns", &model.BoardColumn{}}, {"board_tasks", &model.BoardTask{}},
		{"knowledge_docs", &model.KnowledgeDoc{}}, {"knowledge_chunks", &model.KnowledgeChunk{}},
		{"vault_keys", &model.VaultKey{}}, {"vault_items", &model.VaultItem{}},
		{"conversations", &model.Conversation{}}, {"messages", &model.Message{}},
		{"agent_tasks", &model.AgentTask{}}, {"drive_files", &model.DriveFile{}},
		{"drive_quota", &model.DriveQuota{}},
	}
	for _, tc := range userScoped {
		if c := countRows(t, db, tc.m, "user_id = ?", uid); c != 0 {
			t.Fatalf("table %s still has %d rows for purged user", tc.name, c)
		}
	}
	if c := countRows(t, db, &model.User{}, "id = ?", uid); c != 0 {
		t.Fatalf("purged user row still exists")
	}
	if c := countRows(t, db, &model.NoteTag{}, "note_id IN ?", []uuid.UUID{liveNote.ID, deadNote.ID}); c != 0 {
		t.Fatalf("note_tags still has %d rows for purged user's notes", c)
	}
	// 磁盘文件已删
	if _, err := os.Stat(drivePath); !os.IsNotExist(err) {
		t.Fatalf("drive disk file still exists after purge")
	}
	if _, err := os.Stat(mediaOriginal); !os.IsNotExist(err) {
		t.Fatalf("attachment disk file still exists after purge")
	}
	if _, err := os.Stat(mediaThumb); !os.IsNotExist(err) {
		t.Fatalf("attachment thumb still exists after purge")
	}

	// ===== 断言：其他用户数据不受影响 =====
	if c := countRows(t, db, &model.User{}, "id = ?", other); c != 1 {
		t.Fatalf("other user was deleted")
	}
	if c := countRows(t, db, &model.Note{}, "id = ?", otherNote.ID); c != 1 {
		t.Fatalf("other user's note was deleted")
	}
}

// TestSoftDeleteFiltersList 锁定「软删除可过滤、数据仍保留、关联行无孤儿」三个语义：
// 删除笔记后列表不可见，但 Unscoped 计数仍为 1（可恢复），且标签关联/稳定 ID 已硬删。
func TestSoftDeleteFiltersList(t *testing.T) {
	db := openIntegrityDB(t)
	uid := uuid.New()
	n := &model.Note{ID: uuid.New(), UserID: uid, Title: "n1", Body: "b"}
	mustCreate(t, db, n)
	tag := &model.Tag{ID: uuid.New(), UserID: uid, Name: "t"}
	mustCreate(t, db, tag)
	mustCreate(t, db, &model.NoteTag{NoteID: n.ID, TagID: tag.ID})
	mustCreate(t, db, &model.NoteStableID{NoteID: n.ID, UserID: uid, Stable: "s-1"})

	rows, total, err := db.ListNotes(NoteListQuery{UserID: uid})
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("before delete: rows=%d total=%d err=%v", len(rows), total, err)
	}

	if err := db.DeleteNote(uid, n.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// 列表不再可见
	if _, total, _ := db.ListNotes(NoteListQuery{UserID: uid}); total != 0 {
		t.Fatalf("after delete: total=%d, want 0", total)
	}
	// 数据仍保留（可恢复）：Unscoped 计数为 1
	if c := countRows(t, db, &model.Note{}, "id = ?", n.ID); c != 1 {
		t.Fatalf("soft-deleted note lost: count=%d, want 1", c)
	}
	// 标签关联与稳定 ID 已硬删（无孤儿）
	if c := countRows(t, db, &model.NoteTag{}, "note_id = ?", n.ID); c != 0 {
		t.Fatalf("note_tags orphaned: %d", c)
	}
	if c := countRows(t, db, &model.NoteStableID{}, "note_id = ?", n.ID); c != 0 {
		t.Fatalf("note_stable_ids orphaned: %d", c)
	}
}

// TestCleanupOrphanRows 锁定孤儿清理的三个层次，且不影响正常数据：
//  1. 用户级孤儿：幽灵用户（ID 从未落库）名下的全套数据（含磁盘文件与配额行）被清空；
//  2. 网盘父目录断链：父目录行丢失后，其子文件经递归 CTE 被收集，行与磁盘一并回收；
//  3. 实体从属孤儿：指向不存在笔记的附件被清理并回收磁盘。
func TestCleanupOrphanRows(t *testing.T) {
	db := openIntegrityDB(t)
	alice := uuid.New()
	bob := uuid.New()
	ghost := uuid.New() // 从未落库的用户 ID：其名下数据全是历史孤儿

	mustCreate(t, db, &model.User{ID: alice, Username: "alice", Email: "a@x.com", PasswordHash: "x", Role: "user", Status: "active"})
	mustCreate(t, db, &model.User{ID: bob, Username: "bob", Email: "b@x.com", PasswordHash: "x", Role: "user", Status: "active"})

	// ---- 层 1 种子：幽灵用户的全套数据 ----
	gNote := &model.Note{ID: uuid.New(), UserID: ghost, Title: "g", Body: "b"}
	mustCreate(t, db, gNote)
	gTag := &model.Tag{ID: uuid.New(), UserID: ghost, Name: "gtag"}
	mustCreate(t, db, gTag)
	mustCreate(t, db, &model.NoteTag{NoteID: gNote.ID, TagID: gTag.ID})
	mustCreate(t, db, &model.NoteStableID{NoteID: gNote.ID, UserID: ghost, Stable: "g-1"})
	gAttPath := db.Media.PathForURL("/uploads/ghost.png")
	if err := os.WriteFile(gAttPath, []byte("g"), 0o644); err != nil {
		t.Fatalf("write ghost attachment: %v", err)
	}
	mustCreate(t, db, &model.Attachment{ID: uuid.New(), UserID: ghost, Filename: "ghost.png",
		URL: "/uploads/ghost.png", MimeType: "image/png"})
	gStored, err := db.Drive.Save(ghost, strings.NewReader("ghost-drive"))
	if err != nil {
		t.Fatalf("drive save: %v", err)
	}
	gDrivePath := db.Drive.Path(ghost, gStored)
	mustCreate(t, db, &model.DriveFile{ID: uuid.New(), UserID: ghost, Name: "g.bin",
		Size: 11, StoredName: gStored, IsDir: false})
	if err := db.EnsureDriveQuotaRow(ghost); err != nil {
		t.Fatalf("ensure quota: %v", err)
	}
	gBoard := &model.Board{ID: uuid.New(), UserID: ghost, Name: "gb"}
	mustCreate(t, db, gBoard)
	mustCreate(t, db, &model.BoardColumn{ID: uuid.New(), BoardID: gBoard.ID, UserID: ghost, Title: "c"})
	mustCreate(t, db, &model.BoardTask{ID: uuid.New(), UserID: ghost, BoardID: gBoard.ID, ColumnID: uuid.New(), Title: "t"})
	gDoc := &model.KnowledgeDoc{ID: uuid.New(), UserID: ghost, Title: "gd"}
	mustCreate(t, db, gDoc)
	mustCreate(t, db, &model.KnowledgeChunk{ID: uuid.New(), DocID: gDoc.ID, UserID: ghost, Content: "c"})
	mustCreate(t, db, &model.VaultKey{ID: uuid.New(), UserID: ghost, Salt: "s", Verifier: "v"})
	mustCreate(t, db, &model.VaultItem{ID: uuid.New(), UserID: ghost, Title: "i", Encrypted: "e"})
	gConv := &model.Conversation{ID: uuid.New(), UserID: ghost, Title: "cv"}
	mustCreate(t, db, gConv)
	mustCreate(t, db, &model.Message{ID: uuid.New(), ConversationID: gConv.ID, UserID: ghost, Role: "user", Content: "m"})
	mustCreate(t, db, &model.AgentTask{ID: uuid.New(), UserID: ghost, Goal: "g", Status: "done"})

	// ---- 层 2 种子：alice 的网盘父目录断链 ----
	// folderY 下的 fileZ；随后硬删 folderY 行，模拟「父目录行丢失」的历史孤儿。
	folderY := &model.DriveFile{ID: uuid.New(), UserID: alice, Name: "folderY", IsDir: true}
	mustCreate(t, db, folderY)
	zStored, err := db.Drive.Save(alice, strings.NewReader("orphan-child"))
	if err != nil {
		t.Fatalf("drive save: %v", err)
	}
	zPath := db.Drive.Path(alice, zStored)
	fileZ := &model.DriveFile{ID: uuid.New(), UserID: alice, ParentID: &folderY.ID,
		Name: "fileZ.bin", Size: 12, StoredName: zStored, IsDir: false}
	mustCreate(t, db, fileZ)
	if err := db.gorm.Unscoped().Delete(folderY).Error; err != nil {
		t.Fatalf("drop folder row: %v", err)
	}

	// ---- 层 3 种子：bob 的附件指向一个从未落库的笔记 ----
	missing := uuid.New()
	bAttPath := db.Media.PathForURL("/uploads/broken.png")
	if err := os.WriteFile(bAttPath, []byte("b"), 0o644); err != nil {
		t.Fatalf("write broken attachment: %v", err)
	}
	mustCreate(t, db, &model.Attachment{ID: uuid.New(), UserID: bob, NoteID: &missing,
		Filename: "broken.png", URL: "/uploads/broken.png", MimeType: "image/png"})

	// ---- 正常数据（清理后必须完好）----
	bNote := &model.Note{ID: uuid.New(), UserID: bob, Title: "bn", Body: "b"}
	mustCreate(t, db, bNote)
	bOKPath := db.Media.PathForURL("/uploads/ok.png")
	if err := os.WriteFile(bOKPath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write ok attachment: %v", err)
	}
	bOK := &model.Attachment{ID: uuid.New(), UserID: bob, NoteID: &bNote.ID,
		Filename: "ok.png", URL: "/uploads/ok.png", MimeType: "image/png"}
	mustCreate(t, db, bOK)
	aOKStored, err := db.Drive.Save(alice, strings.NewReader("alive"))
	if err != nil {
		t.Fatalf("drive save: %v", err)
	}
	aOKPath := db.Drive.Path(alice, aOKStored)
	aOK := &model.DriveFile{ID: uuid.New(), UserID: alice, Name: "alive.bin",
		Size: 5, StoredName: aOKStored, IsDir: false}
	mustCreate(t, db, aOK)

	cleaned, err := db.CleanupOrphanRows()
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	// 幽灵用户 16 表 16 行 + note_tags 1 + 断链 fileZ 1 + 断链附件 1 = 19
	if cleaned < 19 {
		t.Fatalf("cleaned = %d, want >= 19", cleaned)
	}

	// ===== 断言：幽灵用户数据清空 =====
	ghostScoped := []struct {
		name string
		m    any
	}{
		{"notes", &model.Note{}}, {"tags", &model.Tag{}}, {"note_stable_ids", &model.NoteStableID{}},
		{"attachments", &model.Attachment{}}, {"boards", &model.Board{}}, {"board_columns", &model.BoardColumn{}},
		{"board_tasks", &model.BoardTask{}}, {"knowledge_docs", &model.KnowledgeDoc{}},
		{"knowledge_chunks", &model.KnowledgeChunk{}}, {"vault_keys", &model.VaultKey{}},
		{"vault_items", &model.VaultItem{}}, {"conversations", &model.Conversation{}},
		{"messages", &model.Message{}}, {"agent_tasks", &model.AgentTask{}},
		{"drive_files", &model.DriveFile{}}, {"drive_quota", &model.DriveQuota{}},
	}
	for _, tc := range ghostScoped {
		if c := countRows(t, db, tc.m, "user_id = ?", ghost); c != 0 {
			t.Fatalf("orphan table %s still has %d rows for ghost user", tc.name, c)
		}
	}
	if c := countRows(t, db, &model.NoteTag{}, "note_id = ?", gNote.ID); c != 0 {
		t.Fatalf("ghost note_tags not cleaned")
	}
	if _, err := os.Stat(gDrivePath); !os.IsNotExist(err) {
		t.Fatalf("ghost drive disk file still exists")
	}
	if _, err := os.Stat(gAttPath); !os.IsNotExist(err) {
		t.Fatalf("ghost attachment disk file still exists")
	}

	// ===== 断言：断链网盘子文件被删（行 + 磁盘） =====
	if c := countRows(t, db, &model.DriveFile{}, "id = ?", fileZ.ID); c != 0 {
		t.Fatalf("broken-parent drive file survived")
	}
	if _, err := os.Stat(zPath); !os.IsNotExist(err) {
		t.Fatalf("broken-parent drive disk file still exists")
	}

	// ===== 断言：note 断链附件被删（行 + 磁盘） =====
	if c := countRows(t, db, &model.Attachment{}, "note_id = ?", missing); c != 0 {
		t.Fatalf("broken-note attachment survived")
	}
	if _, err := os.Stat(bAttPath); !os.IsNotExist(err) {
		t.Fatalf("broken-note attachment disk file still exists")
	}

	// ===== 断言：正常用户数据完好 =====
	if c := countRows(t, db, &model.User{}, "id IN ?", []uuid.UUID{alice, bob}); c != 2 {
		t.Fatalf("real users damaged: %d", c)
	}
	if c := countRows(t, db, &model.Note{}, "id = ?", bNote.ID); c != 1 {
		t.Fatalf("bob note damaged")
	}
	if c := countRows(t, db, &model.Attachment{}, "id = ?", bOK.ID); c != 1 {
		t.Fatalf("bob attachment damaged")
	}
	if _, err := os.Stat(bOKPath); err != nil {
		t.Fatalf("bob attachment disk file damaged: %v", err)
	}
	if c := countRows(t, db, &model.DriveFile{}, "id = ?", aOK.ID); c != 1 {
		t.Fatalf("alice drive file damaged")
	}
	if _, err := os.Stat(aOKPath); err != nil {
		t.Fatalf("alice drive disk file damaged: %v", err)
	}
}

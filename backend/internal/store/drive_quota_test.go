package store

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"starry/backend/internal/model"
)

func openDriveSQLite(t *testing.T) *DB {
	t.Helper()
	g, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// 单连接 + 共享缓存：保证内存库在连接池与并发 goroutine 间一致可见。
	sqlDB, err := g.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := g.AutoMigrate(&model.DriveFile{}, &model.DriveQuota{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewFromGorm(g)
}

// TestEnsureDriveQuotaRowBackfills 验证老用户（已有文件但无配额行）首次
// 确保配额行时按已有文件体积回填，使 used 与文件行初始一致。
func TestEnsureDriveQuotaRowBackfills(t *testing.T) {
	db := openDriveSQLite(t)
	uid := uuid.New()
	if err := db.InsertDriveFile(&model.DriveFile{UserID: uid, Name: "a", Size: 300, IsDir: false}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.InsertDriveFile(&model.DriveFile{UserID: uid, Name: "b", Size: 200, IsDir: false}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.EnsureDriveQuotaRow(uid); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	q, err := db.GetOrCreateDriveQuota(uid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if q.Used != 500 {
		t.Fatalf("backfilled used = %d, want 500", q.Used)
	}
}

// TestClaimDriveSpaceRejectsWhenExceeds 验证原子占用：仅在 used+size<=quota
// 时才成功，越过配额必被拒绝（单连接下也锁定了「查-改」不可被拆开）。
func TestClaimDriveSpaceRejectsWhenExceeds(t *testing.T) {
	db := openDriveSQLite(t)
	uid := uuid.New()
	if err := db.EnsureDriveQuotaRow(uid); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	const quota int64 = 1000

	if ok, err := db.ClaimDriveSpace(uid, 600, quota); err != nil || !ok {
		t.Fatalf("first claim: ok=%v err=%v", ok, err)
	}
	if ok, err := db.ClaimDriveSpace(uid, 500, quota); err != nil || ok {
		t.Fatalf("over-quota claim should fail: ok=%v err=%v", ok, err)
	}
	if ok, err := db.ClaimDriveSpace(uid, 400, quota); err != nil || !ok {
		t.Fatalf("remaining claim: ok=%v err=%v", ok, err)
	}
	q, _ := db.GetOrCreateDriveQuota(uid)
	if q.Used != 1000 {
		t.Fatalf("used = %d, want 1000", q.Used)
	}
}

// TestClaimDriveSpaceConcurrentNoOvershoot 是配额竞态的精确回归：并发发起
// 远超配额的占用请求，断言「成功占用的总量」永不越过 quota。ClaimDriveSpace
// 的单条带条件 UPDATE 在数据库层面串行化，从根本上消除 TOCTOU 超发。
func TestClaimDriveSpaceConcurrentNoOvershoot(t *testing.T) {
	db := openDriveSQLite(t)
	uid := uuid.New()
	if err := db.EnsureDriveQuotaRow(uid); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	const quota int64 = 1000
	const per = int64(100)
	const goroutines = 20

	var success int64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			ok, err := db.ClaimDriveSpace(uid, per, quota)
			if err != nil {
				t.Errorf("claim error: %v", err)
				return
			}
			if ok {
				atomic.AddInt64(&success, per)
			}
		}()
	}
	wg.Wait()

	if success > quota {
		t.Fatalf("overshoot: successful claims total = %d, quota = %d", success, quota)
	}
	q, _ := db.GetOrCreateDriveQuota(uid)
	if q.Used != success {
		t.Fatalf("used = %d, but successful claims = %d (must match)", q.Used, success)
	}
}

// TestDeleteDriveFileAndReclaim 验证删除文件元数据行时在同一事务内归还配额，
// 不会留下「文件没了却仍占着空间」的漂移。流程对齐真实 handler：
// 先 Ensure→Claim 占用，再 InsertDriveFile 落库，最后删除并回写配额。
func TestDeleteDriveFileAndReclaim(t *testing.T) {
	db := openDriveSQLite(t)
	uid := uuid.New()
	if err := db.EnsureDriveQuotaRow(uid); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if ok, err := db.ClaimDriveSpace(uid, 400, 1000); err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}
	f := &model.DriveFile{UserID: uid, Name: "big", Size: 400, IsDir: false}
	if err := db.InsertDriveFile(f); err != nil {
		t.Fatalf("insert: %v", err)
	}
	q, _ := db.GetOrCreateDriveQuota(uid)
	if q.Used != 400 {
		t.Fatalf("used before delete = %d, want 400", q.Used)
	}
	if _, err := db.DeleteDriveFileAndReclaim(uid, f.ID); err != nil {
		t.Fatalf("delete+reclaim: %v", err)
	}
	q, _ = db.GetOrCreateDriveQuota(uid)
	if q.Used != 0 {
		t.Fatalf("used after delete = %d, want 0", q.Used)
	}
	// 重复删除应安全（文件已不存在），且不再改变 used。
	if _, err := db.DeleteDriveFileAndReclaim(uid, f.ID); err != nil {
		t.Fatalf("repeat delete: %v", err)
	}
	q, _ = db.GetOrCreateDriveQuota(uid)
	if q.Used != 0 {
		t.Fatalf("used after repeat delete = %d, want 0", q.Used)
	}
}

package store

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"starry/backend/internal/model"
)

func openSQLite(t *testing.T) *DB {
	t.Helper()
	g, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := g.AutoMigrate(&model.Note{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewFromGorm(g)
}

// TestTransactionRollsBackOnError 锁定回滚原语：事务内写入后 fn 返回错误，
// 已写入的笔记必须整体回滚、不得落库。这是「导入不产生孤儿笔记」的底层保证。
func TestTransactionRollsBackOnError(t *testing.T) {
	db := openSQLite(t)
	uid := uuid.New()

	err := db.Transaction(func(tx *DB) error {
		n := &model.Note{UserID: uid, Title: "should-not-persist"}
		if e := tx.CreateNote(n); e != nil {
			return e
		}
		return errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected transaction to surface the inner error")
	}

	var count int64
	if e := db.gorm.Model(&model.Note{}).Where("user_id = ?", uid).Count(&count).Error; e != nil {
		t.Fatalf("count: %v", e)
	}
	if count != 0 {
		t.Fatalf("rolled-back note leaked: count=%d, want 0", count)
	}
}

// TestTransactionCommitsOnSuccess 确认成功路径正常提交。
func TestTransactionCommitsOnSuccess(t *testing.T) {
	db := openSQLite(t)
	uid := uuid.New()

	err := db.Transaction(func(tx *DB) error {
		n := &model.Note{UserID: uid, Title: "persisted"}
		return tx.CreateNote(n)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var count int64
	db.gorm.Model(&model.Note{}).Where("user_id = ?", uid).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 note, got %d", count)
	}
}

// TestWithTxReusesMethods 确认 WithTx 能复用既有读写入事务，无需改动方法签名。
func TestWithTxReusesMethods(t *testing.T) {
	db := openSQLite(t)
	uid := uuid.New()
	err := db.gorm.Transaction(func(g *gorm.DB) error {
		tx := db.WithTx(g)
		n := &model.Note{UserID: uid, Title: "via-tx"}
		return tx.CreateNote(n)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var count int64
	db.gorm.Model(&model.Note{}).Where("user_id = ?", uid).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 note, got %d", count)
	}
}

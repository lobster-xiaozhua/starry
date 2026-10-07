package vault

import (
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立保险箱模块拥有的表：主密钥材料（盐/verifier）与密文条目。
//
// 条目列表按更新时间倒序返回，故补一条复合索引；AutoMigrate 只会给出单列 user_id 索引。
func Migrate(db *store.DB) error {
	if err := db.AutoMigrate(&model.VaultKey{}, &model.VaultItem{}); err != nil {
		return err
	}
	return db.ExecDDL(
		"CREATE INDEX IF NOT EXISTS idx_vault_items_user_updated ON vault_items(user_id, updated_at DESC)",
	)
}

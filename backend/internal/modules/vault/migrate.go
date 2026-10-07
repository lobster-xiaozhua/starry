package vault

import (
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立保险箱模块拥有的表：主密钥材料（盐/verifier）与密文条目。
func Migrate(db *store.DB) error {
	return db.AutoMigrate(&model.VaultKey{}, &model.VaultItem{})
}

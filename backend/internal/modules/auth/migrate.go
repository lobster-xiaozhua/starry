package auth

import (
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立认证模块拥有的表：用户与认证安全参数。
// 用户表被多个模块外键引用，故在编排顺序上最先迁移（见 modules.MigrateAll）。
func Migrate(db *store.DB) error {
	return db.AutoMigrate(&model.User{}, &model.AuthSettings{})
}

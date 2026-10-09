package auth

import (
	"starry/backend/internal/migration"
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立认证模块拥有的表：用户与认证安全参数。
// 用户表被多个模块外键引用，故在编排顺序上最先迁移（见 modules.MigrateAll）。
//
// 渐进切换：若 schema_migrations 表存在且包含 0001 版本，说明
// golang-migrate 已接管 users/auth_settings 的建表，则跳过 AutoMigrate；
// 否则回退到 AutoMigrate（兼容已有数据库，无需手动迁移即可升级）。
func Migrate(db *store.DB) error {
	if migration.HasMigrationVersion(db.Gorm(), 1) {
		return nil
	}
	return db.AutoMigrate(&model.User{}, &model.AuthSettings{})
}

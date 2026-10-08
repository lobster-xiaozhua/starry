// Package modules 是业务模块的注册表（模块清单 + 统一装配）。
//
// 每个业务模块是一个独立包（auth / admin / knowledge / notes / chat / boards /
// drive / vault），各自暴露 Register 完成路由挂载与自身服务装配。本包只负责：
//   - 声明模块清单与注册顺序；
//   - 依据环境变量开关决定启用哪些模块（精细调控旋钮）；
//   - 向组合根汇报启用/停用情况。
//
// 模块之间不互相 import，因此任一模块可独立开关、演进甚至删除。
package modules

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/modules/admin"
	"starry/backend/internal/modules/auth"
	"starry/backend/internal/modules/boards"
	"starry/backend/internal/modules/chat"
	"starry/backend/internal/modules/drive"
	"starry/backend/internal/modules/knowledge"
	"starry/backend/internal/modules/notes"
	"starry/backend/internal/modules/vault"
	"starry/backend/internal/store"
)

// Module 描述一个业务模块。
type Module struct {
	Name     string // 模块名，对应开关环境变量 MODULES_<NAME>_ENABLED
	Register func(api *gin.RouterGroup, d *core.Deps)

	// Migrate 可选：迁移本模块拥有的表。为 nil 表示本模块不拥有独立表
	// （例如 admin 只操作 auth 拥有的用户与设置表），不参与迁移编排。
	Migrate func(db *store.DB) error
}

// All 返回模块清单，顺序即路由注册与迁移执行顺序。
// auth 居首是因为用户表被其余模块引用；knowledge 需要 pgvector 扩展，
// 扩展由 MigrateAll 在所有模块之前统一创建。
func All() []Module {
	return []Module{
		{Name: "auth", Register: auth.Register, Migrate: auth.Migrate},
		{Name: "admin", Register: admin.Register, Migrate: nil},
		{Name: "knowledge", Register: knowledge.Register, Migrate: knowledge.Migrate},
		{Name: "notes", Register: notes.Register, Migrate: notes.Migrate},
		{Name: "chat", Register: chat.Register, Migrate: chat.Migrate},
		{Name: "boards", Register: boards.Register, Migrate: boards.Migrate},
		{Name: "drive", Register: drive.Register, Migrate: drive.Migrate},
		{Name: "vault", Register: vault.Register, Migrate: vault.Migrate},
	}
}

// MigrateAll 按模块顺序执行 schema 迁移：先创建依赖扩展（pgvector / pg_trgm / pgcrypto），
// 再由各模块迁移自己拥有的表。
//
// 迁移与路由共用同一套开关：模块被停用时既不挂载路由、也不迁移其表，
// 因此「关掉一个模块」是彻底且一致的。此处只做编排拆分，不重命名既有表。
func MigrateAll(db *store.DB) error {
	if err := db.EnsureExtensions(); err != nil {
		return fmt.Errorf("create extensions failed: %w", err)
	}
	// 审计表是跨模块的安全基础设施（登录成败、锁定、管理操作都会落库），
	// 必须在核心步骤统一迁移，与任何业务模块开关解耦：否则关掉 admin 后
	// 登录审计的落库会因缺表而失败。
	if err := db.MigrateAudit(); err != nil {
		return fmt.Errorf("migrate audit failed: %w", err)
	}
	for _, m := range All() {
		if !Enabled(m.Name) || m.Migrate == nil {
			continue
		}
		if err := m.Migrate(db); err != nil {
			return fmt.Errorf("module %s migration failed: %w", m.Name, err)
		}
	}
	return nil
}

// RegisterAll 装配所有启用的模块，返回启用与停用的模块名，供启动日志汇报。
// 默认全部启用；可用 MODULES_<NAME>_ENABLED=false 单独停用某个模块（例如
// MODULES_CHAT_ENABLED=false 关闭 AI 对话能力），路由将完全不挂载。
func RegisterAll(api *gin.RouterGroup, d *core.Deps) (enabled, disabled []string) {
	for _, m := range All() {
		if !Enabled(m.Name) {
			disabled = append(disabled, m.Name)
			continue
		}
		m.Register(api, d)
		enabled = append(enabled, m.Name)
	}
	slog.Info("modules enabled", "modules", enabled)
	if len(disabled) > 0 {
		slog.Info("modules disabled by config", "modules", disabled)
	}
	return enabled, disabled
}

// Enabled 判断模块是否启用：环境变量 MODULES_<NAME>_ENABLED 为 false 时停用，
// 未设置或解析失败时默认启用（保证新增模块不会因漏配而静默消失）。
func Enabled(name string) bool {
	v := os.Getenv("MODULES_" + strings.ToUpper(name) + "_ENABLED")
	if v == "" {
		return true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		slog.Warn("invalid module switch, defaulting to enabled",
			"module", name, "env", "MODULES_"+strings.ToUpper(name)+"_ENABLED", "value", v)
		return true
	}
	return b
}

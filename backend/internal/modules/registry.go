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
	"log"
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
)

// Module 描述一个业务模块。
type Module struct {
	Name     string // 模块名，对应开关环境变量 MODULES_<NAME>_ENABLED
	Register func(api *gin.RouterGroup, d *core.Deps)
}

// All 返回模块清单，顺序即路由注册顺序。
func All() []Module {
	return []Module{
		{Name: "auth", Register: auth.Register},
		{Name: "admin", Register: admin.Register},
		{Name: "knowledge", Register: knowledge.Register},
		{Name: "notes", Register: notes.Register},
		{Name: "chat", Register: chat.Register},
		{Name: "boards", Register: boards.Register},
		{Name: "drive", Register: drive.Register},
		{Name: "vault", Register: vault.Register},
	}
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
	log.Printf("[modules] enabled: %v", enabled)
	if len(disabled) > 0 {
		log.Printf("[modules] disabled by config: %v", disabled)
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
		log.Printf("[modules] invalid MODULES_%s_ENABLED=%q, defaulting to enabled", strings.ToUpper(name), v)
		return true
	}
	return b
}

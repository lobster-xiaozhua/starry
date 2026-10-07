package core

import (
	"github.com/mojocn/base64Captcha"

	"starry/backend/internal/config"
	"starry/backend/internal/service"
	"starry/backend/internal/sse"
	"starry/backend/internal/store"
)

// Deps 是装配各业务模块所需的共享依赖容器。由组合根（cmd/server）一次性构建，
// 再交给每个模块的 Register 使用。
//
// 设计约束：模块只从 Deps 取用基础设施（DB / Redis / 配置 / 存储 / 中间件所需密钥），
// 业务服务（如 AuthService、NotesService）由各模块在自身 Register 内自行构造，
// 因此模块与模块之间不产生任何 import 依赖——这是模块可独立开关与演进的前提。
type Deps struct {
	Cfg      *config.Config
	DB       *store.DB
	Redis    *store.Redis
	Broker   *sse.Broker
	Media    *store.FileStore         // 笔记附件存储
	Drive    *store.DriveStore        // 网盘文件存储
	Settings *service.SettingsService // 系统设置（密码策略等，跨 auth/admin 共享）
	Captcha  *base64Captcha.Captcha   // 验证码生成器
}

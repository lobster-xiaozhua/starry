// Package vault 保险箱模块：零知识加密 vault 的盐值、初始化与密文条目管理。
// 主密码与明文永不上传服务端，服务端只保存盐值与密文（详见 frontend/src/crypto.ts）。
package vault

import (
	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
)

// Register 装配并挂载保险箱模块路由。
func Register(api *gin.RouterGroup, d *core.Deps) {
	h := New(d.DB)

	g := api.Group("/vault", middleware.JWTAuth(d.Cfg.JWTSecret))
	{
		g.GET("/salt", h.GetSalt)
		g.POST("/setup", h.Setup)
		g.GET("/items", h.ListItems)
		g.POST("/items", h.CreateItem)
		g.PATCH("/items/:id", h.UpdateItem)
		g.DELETE("/items/:id", h.DeleteItem)
	}
}

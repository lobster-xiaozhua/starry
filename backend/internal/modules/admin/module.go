// Package admin 管理模块：系统安全参数（设置）与用户管理（解锁/冻结/改密/吊销会话）。
// 全部端点要求管理员角色。
package admin

import (
	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
)

// Register 装配并挂载管理模块路由。
func Register(api *gin.RouterGroup, d *core.Deps) {
	h := New(d.Settings, NewService(d.DB, d.Redis))

	g := api.Group("/admin", middleware.JWTAuth(d.Cfg.JWTSecret), middleware.RequireAdmin())
	{
		g.GET("/settings", h.GetSettings)
		g.PUT("/settings", h.UpdateSettings)
		g.GET("/users", h.ListUsers)
		g.GET("/users/:id", h.GetUser)
		g.POST("/users/:id/unlock", h.UnlockUser)
		g.POST("/users/:id/freeze", h.FreezeUser)
		g.POST("/users/:id/unfreeze", h.UnfreezeUser)
		g.POST("/users/:id/reset-password", h.ForceResetPassword)
		g.POST("/users/:id/revoke-sessions", h.RevokeSessions)
	}
}

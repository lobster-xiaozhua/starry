// Package drive 网盘模块：文件夹与文件的列举、上传、下载、重命名与删除。
// 下载强制 attachment + nosniff，防止用户上传的 HTML 被同源渲染（存储型 XSS）。
package drive

import (
	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
)

// Register 装配并挂载网盘模块路由。
func Register(api *gin.RouterGroup, d *core.Deps) {
	h := New(d.Cfg, d.DB, d.Drive)

	g := api.Group("/drive", middleware.JWTAuth(d.Cfg.JWTSecret))
	{
		g.GET("", h.List)
		g.POST("/folders", h.CreateFolder)
		g.POST("/upload", h.Upload)
		g.GET("/:id/download", h.Download)
		g.PATCH("/:id", h.Rename)
		g.DELETE("/:id", h.Delete)
	}
}

// Package notes 云笔记模块：笔记 CRUD、标签、导入导出、附件上传与 SSE 实时事件。
package notes

import (
	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
	"starry/backend/internal/service"
)

// Register 装配并挂载云笔记模块路由。
func Register(api *gin.RouterGroup, d *core.Deps) {
	h := New(service.NewNotesService(d.DB), d.Media, d.Broker)

	g := api.Group("/notes", middleware.JWTAuth(d.Cfg.JWTSecret), middleware.RequireUser())
	{
		g.GET("", h.List)
		g.POST("", h.Create)
		g.GET("/tags", h.Tags)
		g.POST("/seed-demo", h.SeedDemo)
		g.POST("/upload", h.Upload)
		g.GET("/export/all", h.ExportAll)
		g.POST("/import", h.Import)
		g.GET("/events", h.Events)
		g.GET("/:id", h.Get)
		g.PUT("/:id", h.Update)
		g.DELETE("/:id", h.Delete)
		g.POST("/:id/archive", h.Archive)
		g.GET("/:id/export", h.ExportMarkdown)
	}
}

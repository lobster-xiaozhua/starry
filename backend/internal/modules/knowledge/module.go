// Package knowledge 知识库（RAG）模块：文档入库、向量检索、列举与删除。
// 向量化由独立 embed 服务完成（见 services/embed），本模块只负责编排与持久化。
package knowledge

import (
	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
)

// Register 装配并挂载知识库模块路由。
func Register(api *gin.RouterGroup, d *core.Deps) {
	h := New(d.DB, d.Cfg)

	g := api.Group("/knowledge", middleware.JWTAuth(d.Cfg.JWTSecret), middleware.RequireUser())
	{
		g.POST("/ingest", h.Ingest)
		g.GET("/search", h.Search)
		g.GET("/docs", h.List)
		g.DELETE("/docs/:id", h.Delete)
	}
}

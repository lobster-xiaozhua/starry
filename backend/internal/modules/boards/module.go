// Package boards 任务看板模块：看板、列与任务的层级管理。
// 路由上拆为 /boards、/columns、/tasks 三组，但同属一个业务域，故收敛在本包内统一注册。
package boards

import (
	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
)

// Register 装配并挂载看板模块路由（看板 / 列 / 任务三组）。
func Register(api *gin.RouterGroup, d *core.Deps) {
	h := New(d.DB)
	auth := middleware.JWTAuth(d.Cfg.JWTSecret)

	boards := api.Group("/boards", auth)
	{
		boards.GET("", h.List)
		boards.POST("", h.Create)
		boards.PATCH("/:id", h.Update)
		boards.DELETE("/:id", h.Delete)
		boards.POST("/:id/columns", h.CreateColumn)
		boards.POST("/:id/tasks", h.CreateTask)
	}

	columns := api.Group("/columns", auth)
	{
		columns.PATCH("/:id", h.UpdateColumn)
		columns.DELETE("/:id", h.DeleteColumn)
	}

	tasks := api.Group("/tasks", auth)
	{
		tasks.PATCH("/:id", h.UpdateTask)
		tasks.DELETE("/:id", h.DeleteTask)
	}
}

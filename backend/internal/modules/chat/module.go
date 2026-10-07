// Package chat AI 对话模块：会话与消息的 CRUD、token 用量上报，以及长程任务的
// 持久化与状态流转。流式推理由 services/agent 承担，本模块负责对话数据的存储层。
package chat

import (
	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
)

// Register 装配并挂载 AI 对话模块路由。
func Register(api *gin.RouterGroup, d *core.Deps) {
	h := New(d.DB, d.Redis)

	g := api.Group("/agent", middleware.JWTAuth(d.Cfg.JWTSecret), middleware.RequireUser())
	{
		g.GET("/conversations", h.ListConversations)
		g.POST("/conversations", h.CreateConversation)
		g.POST("/conversations/seed-demo", h.SeedDemo)
		g.PATCH("/conversations/:id", h.RenameConversation)
		g.DELETE("/conversations/:id", h.DeleteConversation)
		g.GET("/conversations/:id/messages", h.ListMessages)
		g.POST("/conversations/:id/messages", h.SaveMessage)
		g.POST("/conversations/:id/messages/delete", h.DeleteMessages)
		g.POST("/usage", h.RecordUsage)
		g.GET("/usage", h.GetUsage)
		g.POST("/tasks", h.CreateTask)
		g.GET("/tasks", h.ListTasks)
		g.GET("/tasks/:id", h.GetTask)
		g.PATCH("/tasks/:id", h.UpdateTask)
		g.POST("/tasks/:id/cancel", h.CancelTask)
	}
}

package console

import (
	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
)

func Register(api *gin.RouterGroup, d *core.Deps) {
	h := New(d.Cfg, d.DB, d.Redis)
	g := api.Group("/console", middleware.JWTAuth(d.Cfg.JWTSecret), middleware.RequireAdmin())
	g.GET("/health", h.GetHealth)
	g.GET("/traffic", h.GetTraffic)
	g.GET("/settings", h.GetSettings)
	g.GET("/users/count", h.GetUserCount)
}

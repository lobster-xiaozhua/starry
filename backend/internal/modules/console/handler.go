package console

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/bootcheck"
	"starry/backend/internal/config"
	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
	"starry/backend/internal/store"
)

type Handler struct {
	cfg *config.Config
	db  *store.DB
	rds *store.Redis
}

func New(cfg *config.Config, db *store.DB, rds *store.Redis) *Handler {
	return &Handler{cfg: cfg, db: db, rds: rds}
}

// HealthReport combines boot-time checks with real traffic stats.
type HealthReport struct {
	Boot    *bootcheck.Result          `json:"boot"`
	Traffic middleware.TrafficSnapshot `json:"traffic"`
	DB      bool                       `json:"db"`
	Redis   bool                       `json:"redis"`
}

// GetHealth returns boot-time check results + current real traffic stats.
// Does NOT perform new synthetic health checks on every call — only boot-time
// checks are self-checks; ongoing monitoring uses real traffic data.
//
// @Summary      系统健康（开机自检 + 基线流量 + DB/Redis 连通性）
// @Tags         console
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} HealthReport
// @Failure      500 {object} map[string]interface{}
// @Router       /console/health [get]
func (h *Handler) GetHealth(c *gin.Context) {
	report := HealthReport{
		Boot:    bootcheck.Get(),
		Traffic: middleware.GetTrafficStats().Snapshot(),
	}
	report.DB = h.db.Ping() == nil
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	report.Redis = h.rds.Ping(ctx) == nil
	core.OK(c, report)
}

// GetTraffic returns only real-time traffic stats (for auto-refresh polling).
//
// @Summary      实时流量统计
// @Tags         console
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} middleware.TrafficSnapshot
// @Failure      500 {object} map[string]interface{}
// @Router       /console/traffic [get]
func (h *Handler) GetTraffic(c *gin.Context) {
	core.OK(c, middleware.GetTrafficStats().Snapshot())
}

// SettingsReport is the project settings overview for the console.
type SettingsReport struct {
	AppEnv        string                   `json:"appEnv"`
	Modules       []bootcheck.ModuleStatus `json:"modules"`
	DriveMaxBytes int64                    `json:"driveMaxBytes"`
	DriveQuota    int64                    `json:"driveQuotaBytes"`
	MaxBodyBytes  int64                    `json:"maxBodyBytes"`
	AgentURL      string                   `json:"agentUrl"`
	EmbedURL      string                   `json:"embedUrl"`
}

// GetSettings returns project feature settings: module toggles and config overview.
//
// @Summary      项目功能设置（模块开关 + 配置概览）
// @Tags         console
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} SettingsReport
// @Failure      500 {object} map[string]interface{}
// @Router       /console/settings [get]
func (h *Handler) GetSettings(c *gin.Context) {
	report := SettingsReport{
		AppEnv:        h.cfg.AppEnv,
		Modules:       moduleStatusFromBootcheck(),
		DriveMaxBytes: h.cfg.DriveMaxBytes,
		DriveQuota:    h.cfg.DriveQuotaBytes,
		MaxBodyBytes:  h.cfg.MaxBodyBytes,
		AgentURL:      h.cfg.AgentURL,
		EmbedURL:      h.cfg.EmbedURL,
	}
	core.OK(c, report)
}

// GetUserCount returns total user count for the console dashboard.
func (h *Handler) GetUserCount(c *gin.Context) {
	var count int64
	h.db.Gorm().Raw("SELECT count(*) FROM users").Scan(&count)
	core.OK(c, gin.H{"totalUsers": count})
}

// moduleStatusFromBootcheck reads the module list from the boot-time
// check result recorded at startup. This avoids importing the modules
// package directly (which would create an import cycle).
func moduleStatusFromBootcheck() []bootcheck.ModuleStatus {
	r := bootcheck.Get()
	if r == nil {
		return []bootcheck.ModuleStatus{}
	}
	return r.Modules
}

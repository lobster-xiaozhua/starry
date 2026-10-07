package handler

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/config"
)

// TestRouteTreeBuilds 不依赖数据库，仅验证路由树能正确注册：
// 重点确认 /drive/:id 与静态子路由 /folders、/upload 并存时不触发 gin 冲突 panic，
// 以及 boards/columns/tasks/vault 各组端点均成功挂载。
func TestRouteTreeBuilds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		JWTSecret:       "test-secret",
		DriveMaxBytes:   50 << 20,
		DriveQuotaBytes: 1 << 30,
	}
	r := gin.New()
	api := r.Group("/api")

	boards := api.Group("/boards")
	{
		boards.GET("", (&BoardHandler{}).List)
		boards.POST("", (&BoardHandler{}).Create)
		boards.PATCH("/:id", (&BoardHandler{}).Update)
		boards.DELETE("/:id", (&BoardHandler{}).Delete)
		boards.POST("/:id/columns", (&BoardHandler{}).CreateColumn)
		boards.POST("/:id/tasks", (&BoardHandler{}).CreateTask)
	}
	columns := api.Group("/columns")
	{
		columns.PATCH("/:id", (&BoardHandler{}).UpdateColumn)
		columns.DELETE("/:id", (&BoardHandler{}).DeleteColumn)
	}
	tasks := api.Group("/tasks")
	{
		tasks.PATCH("/:id", (&BoardHandler{}).UpdateTask)
		tasks.DELETE("/:id", (&BoardHandler{}).DeleteTask)
	}

	drive := api.Group("/drive")
	{
		drive.GET("", (&DriveHandler{cfg: cfg}).List)
		drive.POST("/folders", (&DriveHandler{cfg: cfg}).CreateFolder)
		drive.POST("/upload", (&DriveHandler{cfg: cfg}).Upload)
		drive.GET("/:id/download", (&DriveHandler{cfg: cfg}).Download)
		drive.PATCH("/:id", (&DriveHandler{cfg: cfg}).Rename)
		drive.DELETE("/:id", (&DriveHandler{cfg: cfg}).Delete)
	}

	vault := api.Group("/vault")
	{
		vault.GET("/salt", (&VaultHandler{}).GetSalt)
		vault.POST("/setup", (&VaultHandler{}).Setup)
		vault.GET("/items", (&VaultHandler{}).ListItems)
		vault.POST("/items", (&VaultHandler{}).CreateItem)
		vault.PATCH("/items/:id", (&VaultHandler{}).UpdateItem)
		vault.DELETE("/items/:id", (&VaultHandler{}).DeleteItem)
	}

	routes := r.Routes()
	if len(routes) == 0 {
		t.Fatal("未注册到任何路由")
	}
	wantPaths := []string{
		"GET /api/boards",
		"POST /api/boards",
		"POST /api/boards/:id/tasks",
		"PATCH /api/columns/:id",
		"PATCH /api/tasks/:id",
		"GET /api/drive",
		"POST /api/drive/folders",
		"POST /api/drive/upload",
		"GET /api/drive/:id/download",
		"GET /api/vault/salt",
		"DELETE /api/vault/items/:id",
	}
	for _, wp := range wantPaths {
		found := false
		for _, rt := range routes {
			if rt.Method+" "+rt.Path == wp {
				found = true
				break
			}
		}
		if !found {
			var got []string
			for _, rt := range routes {
				got = append(got, rt.Method+" "+rt.Path)
			}
			t.Errorf("缺少路由 %s；已注册：%s", wp, strings.Join(got, ", "))
		}
	}
}

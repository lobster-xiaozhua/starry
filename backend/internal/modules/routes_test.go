package modules

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/config"
	"starry/backend/internal/modules/boards"
	"starry/backend/internal/modules/drive"
	"starry/backend/internal/modules/vault"
)

// TestRouteTreeBuilds 不依赖数据库，仅验证跨模块路由树能正确注册：
// 重点确认 /drive/:id 与静态子路由 /folders、/upload 并存时不触发 gin 冲突 panic，
// 以及 boards/columns/tasks/vault 各组端点均成功挂载。
//
// 本测试位于 modules 包（注册表层），因为被验证的行为是「多个模块在同一棵路由树上
// 共存」，而非某个模块的内部逻辑。各模块 Handler 通过公开构造函数 New 获取，
// 传 nil 依赖即可——路由注册阶段不会触达 DB。
func TestRouteTreeBuilds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		JWTSecret:       "test-secret",
		DriveMaxBytes:   50 << 20,
		DriveQuotaBytes: 1 << 30,
	}
	r := gin.New()
	api := r.Group("/api")

	bh := boards.New(nil)
	boardsG := api.Group("/boards")
	{
		boardsG.GET("", bh.List)
		boardsG.POST("", bh.Create)
		boardsG.PATCH("/:id", bh.Update)
		boardsG.DELETE("/:id", bh.Delete)
		boardsG.POST("/:id/columns", bh.CreateColumn)
		boardsG.POST("/:id/tasks", bh.CreateTask)
	}
	columns := api.Group("/columns")
	{
		columns.PATCH("/:id", bh.UpdateColumn)
		columns.DELETE("/:id", bh.DeleteColumn)
	}
	tasks := api.Group("/tasks")
	{
		tasks.PATCH("/:id", bh.UpdateTask)
		tasks.DELETE("/:id", bh.DeleteTask)
	}

	dh := drive.New(cfg, nil, nil)
	driveG := api.Group("/drive")
	{
		driveG.GET("", dh.List)
		driveG.POST("/folders", dh.CreateFolder)
		driveG.POST("/upload", dh.Upload)
		driveG.GET("/:id/download", dh.Download)
		driveG.PATCH("/:id", dh.Rename)
		driveG.DELETE("/:id", dh.Delete)
	}

	vh := vault.New(nil)
	vaultG := api.Group("/vault")
	{
		vaultG.GET("/salt", vh.GetSalt)
		vaultG.POST("/setup", vh.Setup)
		vaultG.GET("/items", vh.ListItems)
		vaultG.POST("/items", vh.CreateItem)
		vaultG.PATCH("/items/:id", vh.UpdateItem)
		vaultG.DELETE("/items/:id", vh.DeleteItem)
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

// TestEnabledDefaultsToTrue 保证模块默认启用，且开关解析符合预期：
// 未配置→启用；false→停用；取值非法→回退启用（避免模块因配置笔误而静默消失）。
func TestEnabledDefaultsToTrue(t *testing.T) {
	t.Setenv("MODULES_CHAT_ENABLED", "")
	if !Enabled("chat") {
		t.Fatal("未设置开关时模块应默认启用")
	}
	t.Setenv("MODULES_CHAT_ENABLED", "false")
	if Enabled("chat") {
		t.Fatal("MODULES_CHAT_ENABLED=false 应停用 chat 模块")
	}
	t.Setenv("MODULES_CHAT_ENABLED", "not-a-bool")
	if !Enabled("chat") {
		t.Fatal("开关取值非法时应回退为启用，避免模块静默消失")
	}
}

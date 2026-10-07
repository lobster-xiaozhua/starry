package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"

	"starry/backend/internal/authpkg"
	"starry/backend/internal/config"
	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
	"starry/backend/internal/modules"
	"starry/backend/internal/service"
	"starry/backend/internal/sse"
	"starry/backend/internal/store"
)

// main 是组合根（composition root）：只负责构建基础设施与共享依赖，
// 再由 modules.RegisterAll 装配各业务模块。业务路由不在本文件出现，
// 新增/下线模块只需改动 modules 注册表与对应模块包，或直接用环境变量开关。
func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal("config invalid: ", err)
	}

	db, err := store.NewPostgres(fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	))
	if err != nil {
		log.Fatal("postgres connection failed: ", err)
	}
	// 迁移由各业务模块自行声明（见 modules.MigrateAll）：扩展先建，随后按模块顺序建表。
	if err := modules.MigrateAll(db); err != nil {
		log.Fatal("migration failed: ", err)
	}
	if err := db.SeedSettings(); err != nil {
		log.Fatal("settings seed failed: ", err)
	}

	rds, err := store.NewRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatal("redis connection failed: ", err)
	}

	seedAdmin(db, cfg.AdminUsername, cfg.AdminEmail, cfg.AdminPassword, cfg.AppEnv)

	// ---- 共享基础设施 ----
	// 系统设置被 auth（密码策略）与 admin（参数维护）共同使用，故作为跨模块共享依赖。
	settingsSvc := service.NewSettingsService(db, rds)

	redisStore := authpkg.NewRedisCaptchaStore(
		func(ctx context.Context, key, value string, ttl time.Duration) error {
			return rds.SetCacheWithTTL(ctx, key, value, ttl)
		},
		func(ctx context.Context, key string) (string, error) {
			return rds.GetString(ctx, key)
		},
		func(ctx context.Context, key string) error { return rds.Delete(ctx, key) },
	)
	captchaGen := base64Captcha.NewCaptcha(base64Captcha.DefaultDriverDigit, redisStore)

	mediaStore := store.NewFileStore(cfg.UploadDir)
	if err := mediaStore.Init(); err != nil {
		log.Fatal("init upload dir failed: ", err)
	}
	driveStore := store.NewDriveStore(cfg.DriveDir)
	if err := driveStore.Init(); err != nil {
		log.Fatal("init drive dir failed: ", err)
	}
	broker := sse.NewBroker(rds.Raw())

	deps := &core.Deps{
		Cfg:      cfg,
		DB:       db,
		Redis:    rds,
		Broker:   broker,
		Media:    mediaStore,
		Drive:    driveStore,
		Settings: settingsSvc,
		Captcha:  captchaGen,
	}

	r := gin.Default()
	r.Use(middleware.CORS(cfg.CorsOrigins))

	api := r.Group("/api")
	// 全部业务路由由各模块自行注册；被停用的模块其路由完全不挂载。
	modules.RegisterAll(api, deps)

	r.Static("/uploads", cfg.UploadDir)

	// 健康检查：供 docker compose / 负载均衡探活。pg 与 redis 任一不可用返回 503。
	r.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "postgres": false, "error": err.Error()})
			return
		}
		if err := rds.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "redis": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "postgres": true, "redis": true})
	})

	// 用 http.Server 包裹 gin 引擎，设置超时并支持优雅关闭。
	// WriteTimeout 不设置（0）：对话/笔记 SSE 流式响应需要长写超时，由各 handler 自行控制；
	// ReadTimeout 限制请求头+体的读取，防止慢速连接耗尽连接池。
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// 监听 SIGINT/SIGTERM，做连接排空后退出，避免滚动更新丢请求。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("server exited")
}

// seedAdmin 首次启动时创建管理员账号。若未提供 ADMIN_PASSWORD：
//   - 开发环境：生成随机密码并打印一次（便于本地首次登录）；
//   - 生产环境：拒绝随机生成，强制要求通过环境变量配置（fail-fast），避免无法登录或弱口令。
//
// 来自 ADMIN_PASSWORD 的密码出于安全不回显明文，仅提示已使用。
func seedAdmin(db *store.DB, username, email, password, appEnv string) {
	existing, err := db.FindUserByUsername(username)
	if err != nil || existing != nil {
		return
	}
	generated := false
	if password == "" {
		if appEnv == "production" {
			log.Fatal("[SEED] 生产环境必须配置 ADMIN_PASSWORD，拒绝以随机密码启动")
		}
		password = generateRandomPassword()
		generated = true
	}
	hash, herr := authpkg.HashPassword(password)
	if herr != nil {
		log.Fatal("admin seed failed: ", herr)
	}
	if err := db.SeedAdmin(username, email, hash); err != nil {
		log.Fatal("admin seed failed: ", err)
	}
	log.Printf("====================================================")
	log.Printf("[SEED] 管理员账号已初始化 username=%s email=%s", username, email)
	if generated {
		log.Printf("[SEED] 初始密码(随机生成，仅打印一次，请立即修改): %s", password)
	} else {
		log.Printf("[SEED] 初始密码来自 ADMIN_PASSWORD 环境变量（出于安全不打印明文）")
	}
	log.Printf("====================================================")
}

func generateRandomPassword() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789!@#$%"
	n, _ := rand.Int(rand.Reader, big.NewInt(8))
	length := 12 + int(n.Int64())
	b := make([]byte, length)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[idx.Int64()]
	}
	return string(b)
}

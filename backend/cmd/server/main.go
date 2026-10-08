package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
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
	"starry/backend/internal/logx"
	"starry/backend/internal/middleware"
	"starry/backend/internal/modules"
	"starry/backend/internal/service"
	"starry/backend/internal/sse"
	"starry/backend/internal/store"
)

// version 由构建时注入（-ldflags -X main.version=...）；本地 go run 时为 dev。
// 通过 /health 暴露，用于确认线上实际运行的构建。
var version = "dev"

// main 是组合根（composition root）：只负责构建基础设施与共享依赖，
// 再由 modules.RegisterAll 装配各业务模块。业务路由不在本文件出现，
// 新增/下线模块只需改动 modules 注册表与对应模块包，或直接用环境变量开关。
func main() {
	cfg := config.Load()
	// 日志要在配置校验之前初始化：校验失败也要有结构化输出，而不是裸 log。
	logx.Setup(cfg.AppEnv, cfg.LogLevel)
	log := logx.L()

	if err := cfg.Validate(); err != nil {
		log.Error("config invalid", "error", err)
		os.Exit(1)
	}

	db, err := store.NewPostgres(fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	))
	if err != nil {
		log.Error("postgres connection failed", "error", err)
		os.Exit(1)
	}
	// 迁移由各业务模块自行声明（见 modules.MigrateAll）：扩展先建，随后按模块顺序建表。
	if err := modules.MigrateAll(db); err != nil {
		log.Error("migration failed", "error", err)
		os.Exit(1)
	}
	if err := db.SeedSettings(cfg.CaptchaEnabled); err != nil {
		log.Error("settings seed failed", "error", err)
		os.Exit(1)
	}

	rds, err := store.NewRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Error("redis connection failed", "error", err)
		os.Exit(1)
	}

	seedAdmin(db, cfg.AdminUsername, cfg.AdminEmail, cfg.AdminPassword, cfg.AppEnv, log)

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
		log.Error("init upload dir failed", "error", err)
		os.Exit(1)
	}
	driveStore := store.NewDriveStore(cfg.DriveDir)
	if err := driveStore.Init(); err != nil {
		log.Error("init drive dir failed", "error", err)
		os.Exit(1)
	}
	// 把两个磁盘存储挂到共享 DB 容器上：销户级联清理（PurgeUserData）需要直接删除
	// 用户的网盘与附件磁盘文件，而级联逻辑必须留在 store 基础设施层以满足模块隔离约束。
	db.Drive = driveStore
	db.Media = mediaStore
	broker := sse.NewBroker(rds.Raw())

	// 限流计数共享于 Redis：多副本部署时额度不会随副本数放大。
	// Redis 不可用时 Limiter 内部 fail-open（见 middleware.Limiter）。
	limiter := middleware.NewLimiter(store.NewRedisCounter(rds))

	deps := &core.Deps{
		Cfg:      cfg,
		DB:       db,
		Redis:    rds,
		Broker:   broker,
		Media:    mediaStore,
		Drive:    driveStore,
		Settings: settingsSvc,
		Captcha:  captchaGen,
		Limiter:  limiter,
	}

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	// 顺序：请求 ID → 恢复 → 访问日志 → 体积上限 → 安全响应头 → CORS。
	// Recovery 放在日志之前，panic 才能被记录成一条完整访问日志而不是丢失。
	// 体积上限刻意放在业务之前：越早拒绝超大请求，白花的内存与 CPU 越少。
	r.Use(
		middleware.RequestID(),
		middleware.Recovery(),
		middleware.RequestLogger("/health"),
		middleware.BodyLimit(cfg.MaxBodyBytes),
		// HSTS 只在生产开启：一旦下发，该域名在 max-age 内的 http 访问会被浏览器强制跳转，
		// 回滚极难；开发环境通常也没有 TLS，下发只会带来困惑。
		middleware.SecurityHeaders(cfg.AppEnv == "production"),
		middleware.CORS(cfg.CorsOrigins),
	)

	api := r.Group("/api")
	// 全部业务路由由各模块自行注册；被停用的模块其路由完全不挂载。
	modules.RegisterAll(api, deps)

	r.Static("/uploads", cfg.UploadDir)

	// 健康检查：供 docker compose / 负载均衡探活。pg 与 redis 任一不可用返回 503。
	// 带上 version：滚动更新后一眼确认跑的是哪个构建，避免「以为发版了其实没发」。
	r.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "postgres": false, "version": version, "error": err.Error()})
			return
		}
		if err := rds.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "redis": false, "version": version, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "postgres": true, "redis": true, "version": version})
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
		log.Info("server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// 监听 SIGINT/SIGTERM，做连接排空后退出，避免滚动更新丢请求。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed, forcing close", "error", err)
		if err := srv.Close(); err != nil {
			log.Error("forced close failed", "error", err)
		}
		os.Exit(1)
	}
	log.Info("server exited")
}

// seedAdmin 首次启动时创建管理员账号。若未提供 ADMIN_PASSWORD：
//   - 开发环境：生成随机密码并打印一次（便于本地首次登录）；
//   - 生产环境：拒绝随机生成，强制要求通过环境变量配置（fail-fast），避免无法登录或弱口令。
//
// 来自 ADMIN_PASSWORD 的密码出于安全不回显明文，仅提示已使用。
func seedAdmin(db *store.DB, username, email, password, appEnv string, log *slog.Logger) {
	existing, err := db.FindUserByUsername(username)
	if err != nil || existing != nil {
		return
	}
	generated := false
	if password == "" {
		if appEnv == "production" {
			log.Error("生产环境必须配置 ADMIN_PASSWORD，拒绝以随机密码启动")
			os.Exit(1)
		}
		password = generateRandomPassword()
		generated = true
	}
	hash, herr := authpkg.HashPassword(password)
	if herr != nil {
		log.Error("admin seed failed: hash", "error", herr)
		os.Exit(1)
	}
	if err := db.SeedAdmin(username, email, hash); err != nil {
		log.Error("admin seed failed", "error", err)
		os.Exit(1)
	}
	log.Info("管理员账号已初始化",
		"username", username, "email", email,
		"password_source", map[bool]string{true: "random", false: "ADMIN_PASSWORD"}[generated])
	if generated {
		// 仅开发环境打印一次明文，生产环境不会走到这里。
		log.Warn("初始密码为随机生成，请登录后立即修改", "password", password)
	}
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

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
	"starry/backend/internal/handler"
	"starry/backend/internal/middleware"
	"starry/backend/internal/service"
	"starry/backend/internal/sse"
	"starry/backend/internal/store"
)

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
	if err := db.AutoMigrate(); err != nil {
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

	authSvc := service.NewAuthService(db, rds, cfg.JWTSecret)
	settingsSvc := service.NewSettingsService(db, rds)
	adminSvc := service.NewAdminService(db, rds)

	redisStore := authpkg.NewRedisCaptchaStore(
		func(ctx context.Context, key, value string, ttl time.Duration) error {
			return rds.SetCacheWithTTL(ctx, key, value, ttl)
		},
		func(ctx context.Context, key string) (string, error) {
			return rds.GetString(ctx, key)
		},
		func(ctx context.Context, key string) error { return rds.Delete(ctx, key) },
	)
	captchaGen := base64Captcha.NewCaptcha(
		base64Captcha.DefaultDriverDigit,
		redisStore,
	)
	authSvc.SetCaptchaVerifier(func(_ context.Context, captchaID, answer string) bool {
		return captchaGen.Verify(captchaID, answer, true)
	})

	authHandler := handler.NewAuthHandler(authSvc, settingsSvc, captchaGen)
	adminHandler := handler.NewAdminHandler(settingsSvc, adminSvc)
	agentHandler := handler.NewAgentHandler(db, rds)
	knowledgeHandler := handler.NewKnowledgeHandler(db, cfg)

	notesSvc := service.NewNotesService(db)
	mediaStore := store.NewFileStore(cfg.UploadDir)
	if err := mediaStore.Init(); err != nil {
		log.Fatal("init upload dir failed: ", err)
	}
	driveStore := store.NewDriveStore(cfg.DriveDir)
	if err := driveStore.Init(); err != nil {
		log.Fatal("init drive dir failed: ", err)
	}
	broker := sse.NewBroker(rds.Raw())
	notesHandler := handler.NewNotesHandler(notesSvc, mediaStore, broker)
	boardHandler := handler.NewBoardHandler(db)
	driveHandler := handler.NewDriveHandler(cfg, db, driveStore)
	vaultHandler := handler.NewVaultHandler(db)

	r := gin.Default()
	r.Use(middleware.CORS(cfg.CorsOrigins))

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/captcha", middleware.RateLimit(20, time.Minute), authHandler.Captcha)
			auth.POST("/login", authHandler.Login)
			auth.POST("/register", authHandler.Register)
			auth.GET("/password-policy", authHandler.PasswordPolicy)
			auth.POST("/forgot-password", authHandler.ForgotPassword)
			auth.POST("/reset-password", authHandler.ResetPassword)
			auth.POST("/refresh", middleware.RateLimit(30, time.Minute), authHandler.Refresh)
			auth.POST("/logout", middleware.JWTAuth(cfg.JWTSecret), authHandler.Logout)
			auth.GET("/me", middleware.JWTAuth(cfg.JWTSecret), authHandler.Me)
		}
		admin := api.Group("/admin", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireAdmin())
		{
			admin.GET("/settings", adminHandler.GetSettings)
			admin.PUT("/settings", adminHandler.UpdateSettings)
			admin.GET("/users", adminHandler.ListUsers)
			admin.GET("/users/:id", adminHandler.GetUser)
			admin.POST("/users/:id/unlock", adminHandler.UnlockUser)
			admin.POST("/users/:id/freeze", adminHandler.FreezeUser)
			admin.POST("/users/:id/unfreeze", adminHandler.UnfreezeUser)
			admin.POST("/users/:id/reset-password", adminHandler.ForceResetPassword)
			admin.POST("/users/:id/revoke-sessions", adminHandler.RevokeSessions)
		}
		knowledge := api.Group("/knowledge", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireUser())
		{
			knowledge.POST("/ingest", knowledgeHandler.Ingest)
			knowledge.GET("/search", knowledgeHandler.Search)
			knowledge.GET("/docs", knowledgeHandler.List)
			knowledge.DELETE("/docs/:id", knowledgeHandler.Delete)
		}

		notes := api.Group("/notes", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireUser())
		{
			notes.GET("", notesHandler.List)
			notes.POST("", notesHandler.Create)
			notes.GET("/tags", notesHandler.Tags)
			notes.POST("/seed-demo", notesHandler.SeedDemo)
			notes.POST("/upload", notesHandler.Upload)
			notes.GET("/export/all", notesHandler.ExportAll)
			notes.POST("/import", notesHandler.Import)
			notes.GET("/events", notesHandler.Events)
			notes.GET("/:id", notesHandler.Get)
			notes.PUT("/:id", notesHandler.Update)
			notes.DELETE("/:id", notesHandler.Delete)
			notes.POST("/:id/archive", notesHandler.Archive)
			notes.GET("/:id/export", notesHandler.ExportMarkdown)
		}

		agent := api.Group("/agent", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireUser())
		{
			agent.GET("/conversations", agentHandler.ListConversations)
			agent.POST("/conversations", agentHandler.CreateConversation)
			agent.POST("/conversations/seed-demo", agentHandler.SeedDemo)
			agent.PATCH("/conversations/:id", agentHandler.RenameConversation)
			agent.DELETE("/conversations/:id", agentHandler.DeleteConversation)
			agent.GET("/conversations/:id/messages", agentHandler.ListMessages)
			agent.POST("/conversations/:id/messages", agentHandler.SaveMessage)
			agent.POST("/conversations/:id/messages/delete", agentHandler.DeleteMessages)
			agent.POST("/usage", agentHandler.RecordUsage)
			agent.GET("/usage", agentHandler.GetUsage)
			agent.POST("/tasks", agentHandler.CreateTask)
			agent.GET("/tasks", agentHandler.ListTasks)
			agent.GET("/tasks/:id", agentHandler.GetTask)
			agent.PATCH("/tasks/:id", agentHandler.UpdateTask)
			agent.POST("/tasks/:id/cancel", agentHandler.CancelTask)
		}

		boards := api.Group("/boards", middleware.JWTAuth(cfg.JWTSecret))
		{
			boards.GET("", boardHandler.List)
			boards.POST("", boardHandler.Create)
			boards.PATCH("/:id", boardHandler.Update)
			boards.DELETE("/:id", boardHandler.Delete)
			boards.POST("/:id/columns", boardHandler.CreateColumn)
			boards.POST("/:id/tasks", boardHandler.CreateTask)
		}
		columns := api.Group("/columns", middleware.JWTAuth(cfg.JWTSecret))
		{
			columns.PATCH("/:id", boardHandler.UpdateColumn)
			columns.DELETE("/:id", boardHandler.DeleteColumn)
		}
		tasks := api.Group("/tasks", middleware.JWTAuth(cfg.JWTSecret))
		{
			tasks.PATCH("/:id", boardHandler.UpdateTask)
			tasks.DELETE("/:id", boardHandler.DeleteTask)
		}

		drive := api.Group("/drive", middleware.JWTAuth(cfg.JWTSecret))
		{
			drive.GET("", driveHandler.List)
			drive.POST("/folders", driveHandler.CreateFolder)
			drive.POST("/upload", driveHandler.Upload)
			drive.GET("/:id/download", driveHandler.Download)
			drive.PATCH("/:id", driveHandler.Rename)
			drive.DELETE("/:id", driveHandler.Delete)
		}

		vault := api.Group("/vault", middleware.JWTAuth(cfg.JWTSecret))
		{
			vault.GET("/salt", vaultHandler.GetSalt)
			vault.POST("/setup", vaultHandler.Setup)
			vault.GET("/items", vaultHandler.ListItems)
			vault.POST("/items", vaultHandler.CreateItem)
			vault.PATCH("/items/:id", vaultHandler.UpdateItem)
			vault.DELETE("/items/:id", vaultHandler.DeleteItem)
		}
	}

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

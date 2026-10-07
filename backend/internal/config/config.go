package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"strconv"
)

type Config struct {
	Port          string
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	JWTSecret     string
	CorsOrigins   string
	AdminUsername string
	AdminEmail    string
	AdminPassword string
	UploadDir     string
	AgentURL      string
	AgentToken    string
	DriveDir      string
	DriveMaxBytes int64
	DriveQuotaBytes int64
}

func Load() *Config {
	jwtSecret := envOr("JWT_SECRET", "")
	if jwtSecret == "" {
		// 未显式配置密钥时回退为随机密钥：每次重启都会让已签发令牌失效，
		// 仅适用于本地开发。生产部署务必通过环境变量注入稳定密钥。
		jwtSecret = generateRandomSecret()
		log.Println("[WARN] JWT_SECRET 未配置，已使用随机密钥；重启后所有会话将失效，生产环境请显式设置")
	}
	return &Config{
		Port:          envOr("PORT", "8080"),
		DBHost:        envOr("DB_HOST", "127.0.0.1"),
		DBPort:        envOr("DB_PORT", "5432"),
		DBUser:        envOr("DB_USER", "login_user"),
		DBPassword:    envOr("DB_PASSWORD", "login_pass_2026"),
		DBName:        envOr("DB_NAME", "login_system"),
		RedisAddr:     envOr("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       0,
		JWTSecret:     jwtSecret,
		CorsOrigins:   envOr("CORS_ORIGINS", ""),
		AdminUsername: envOr("ADMIN_USERNAME", "admin"),
		AdminEmail:    envOr("ADMIN_EMAIL", "admin@example.com"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		UploadDir:     envOr("UPLOAD_DIR", "uploads"),
		AgentURL:      envOr("AGENT_URL", "http://agent:3001"),
		AgentToken:    os.Getenv("AGENT_INTERNAL_TOKEN"),
		DriveDir:      envOr("DRIVE_DIR", "drive"),
		DriveMaxBytes: envOrInt("DRIVE_MAX_BYTES", 50<<20),
		DriveQuotaBytes: envOrInt("DRIVE_QUOTA_BYTES", 1<<30),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func generateRandomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatal("failed to generate JWT secret: ", err)
	}
	return hex.EncodeToString(b)
}

package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strconv"
)

type Config struct {
	AppEnv          string // development | production；影响校验严格程度
	Port            string
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	DBSSLMode       string // disable | require | verify-full；连接托管 PG 时必须设为 require
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	JWTSecret       string
	CorsOrigins     string
	AdminUsername   string
	AdminEmail      string
	AdminPassword   string
	UploadDir       string
	AgentURL        string
	AgentToken      string
	EmbedURL        string
	DriveDir        string
	DriveMaxBytes   int64
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
		AppEnv:          envOr("APP_ENV", "development"),
		Port:            envOr("PORT", "8080"),
		DBHost:          envOr("DB_HOST", "127.0.0.1"),
		DBPort:          envOr("DB_PORT", "5432"),
		DBUser:          envOr("DB_USER", "login_user"),
		DBPassword:      os.Getenv("DB_PASSWORD"),
		DBName:          envOr("DB_NAME", "login_system"),
		DBSSLMode:       envOr("DB_SSLMODE", "disable"),
		RedisAddr:       envOr("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		RedisDB:         0,
		JWTSecret:       jwtSecret,
		CorsOrigins:     envOr("CORS_ORIGINS", ""),
		AdminUsername:   envOr("ADMIN_USERNAME", "admin"),
		AdminEmail:      envOr("ADMIN_EMAIL", "admin@example.com"),
		AdminPassword:   os.Getenv("ADMIN_PASSWORD"),
		UploadDir:       envOr("UPLOAD_DIR", "uploads"),
		AgentURL:        envOr("AGENT_URL", "http://agent:3001"),
		AgentToken:      os.Getenv("AGENT_INTERNAL_TOKEN"),
		EmbedURL:        envOr("EMBED_URL", "http://embed:3002"),
		DriveDir:        envOr("DRIVE_DIR", "drive"),
		DriveMaxBytes:   envOrInt("DRIVE_MAX_BYTES", 50<<20),
		DriveQuotaBytes: envOrInt("DRIVE_QUOTA_BYTES", 1<<30),
	}
}

// Validate 校验关键配置。生产环境（APP_ENV=production）下，缺失数据库连接信息与
// JWT 密钥会导致启动失败（fail-fast）；开发环境仅对弱配置告警，便于本地快速启动。
func (c *Config) Validate() error {
	if c.AppEnv == "production" {
		if c.DBHost == "" || c.DBUser == "" || c.DBName == "" {
			return errors.New("生产环境必须配置 DB_HOST / DB_USER / DB_NAME")
		}
		if c.DBPassword == "" {
			return errors.New("生产环境必须配置 DB_PASSWORD，禁止留空")
		}
		if c.JWTSecret == "" {
			return errors.New("生产环境必须配置 JWT_SECRET，禁止留空（否则会话可被伪造）")
		}
		if c.CorsOrigins == "" || c.CorsOrigins == "*" {
			return errors.New("生产环境必须配置明确的 CORS_ORIGINS，禁止使用 * 或留空（凭据跨站泄露风险）")
		}
		return nil
	}
	// 开发环境：仅告警，不阻断启动。
	if c.DBPassword == "" {
		log.Println("[WARN] DB_PASSWORD 为空，本地开发可直接连无密码数据库；生产务必配置")
	}
	if c.JWTSecret == "" {
		log.Println("[WARN] JWT_SECRET 为空，已回退随机密钥；生产务必配置稳定密钥")
	}
	return nil
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

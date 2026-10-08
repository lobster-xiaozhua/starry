package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
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
	LogLevel        string // debug | info | warn | error；为空时按环境推导
	CaptchaEnabled  bool   // 首次初始化时写入的验证码开关；之后由管理端设置接管
	MaxBodyBytes    int64  // 常规 JSON 接口的请求体上限；上传类接口在 handler 内按需抬高
}

func Load() *Config {
	jwtSecret := envOr("JWT_SECRET", "")
	if jwtSecret == "" {
		// 未显式配置密钥时回退为随机密钥：每次重启都会让已签发令牌失效，
		// 仅适用于本地开发。生产部署务必通过环境变量注入稳定密钥。
		jwtSecret = generateRandomSecret()
		slog.Warn("JWT_SECRET 未配置，已使用随机密钥；重启后所有会话将失效，生产环境请显式设置")
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
		LogLevel:        envOr("LOG_LEVEL", ""),
		CaptchaEnabled:  envOrBool("CAPTCHA_ENABLED", true),
		MaxBodyBytes:    envOrInt("MAX_BODY_BYTES", 8<<20),
	}
}

// minBodyBytes 是常规 JSON 接口请求体下限的 sanity floor：过小的上限会让所有正常请求
// 都被 413 拒绝，是明显误配置，故在校验阶段强制（无论环境）。
const minBodyBytes int64 = 1024

// allowedLogLevels 与 internal/logx.parseLevel 的已知级别保持一致。未知级别会被
// logx 静默回退为 info，因此这里只告警、不阻断。
var allowedLogLevels = map[string]bool{
	"": true, "debug": true, "info": true, "warn": true, "warning": true, "error": true,
}

// Validate 校验关键配置。生产环境（APP_ENV=production）下，缺失数据库连接信息、JWT 密钥、
// 非法端口、请求体上限、网盘配额关系等会导致启动失败（fail-fast）；开发环境仅对弱配置告警，
// 便于本地快速启动。
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
		// 数值型与格式型 sanity 检查：这类误配置会在运行时静默失效或产生矛盾，必须 fail-fast。
		if err := c.validateSanity(); err != nil {
			return err
		}
		if err := validateCORSOrigins(c.CorsOrigins); err != nil {
			return err
		}
		return nil
	}
	// 开发环境：仅告警，不阻断启动。
	if c.DBPassword == "" {
		slog.Warn("DB_PASSWORD 为空，本地开发可直接连无密码数据库；生产务必配置")
	}
	if c.JWTSecret == "" {
		slog.Warn("JWT_SECRET 为空，已回退随机密钥；生产务必配置稳定密钥")
	}
	c.warnSanity()
	warnCORSOrigins(c.CorsOrigins)
	return nil
}

// validateSanity 检查「无论环境都应有意义」的数值型配置，返回首个致命错误（生产用）。
func (c *Config) validateSanity() error {
	if !validPort(c.Port) {
		return fmt.Errorf("PORT 非法: %q，应为 1-65535 之间的整数", c.Port)
	}
	if c.MaxBodyBytes < minBodyBytes {
		return fmt.Errorf("MAX_BODY_BYTES 过小(%d)，低于下限 %d 字节，正常请求会被全部 413 拒绝", c.MaxBodyBytes, minBodyBytes)
	}
	if c.DriveMaxBytes > c.DriveQuotaBytes {
		return fmt.Errorf("DRIVE_MAX_BYTES(%d) 大于 DRIVE_QUOTA_BYTES(%d)，单文件上限不应超过总配额", c.DriveMaxBytes, c.DriveQuotaBytes)
	}
	return nil
}

// warnSanity 与 validateSanity 同源，但开发环境只告警、不阻断启动。
func (c *Config) warnSanity() {
	if !validPort(c.Port) {
		slog.Warn("PORT 非法，启动后可能无法监听", "port", c.Port)
	}
	if c.MaxBodyBytes < minBodyBytes {
		slog.Warn("MAX_BODY_BYTES 过小，正常请求可能被全部 413 拒绝", "value", c.MaxBodyBytes, "min", minBodyBytes)
	}
	if c.DriveMaxBytes > c.DriveQuotaBytes {
		slog.Warn("DRIVE_MAX_BYTES 大于 DRIVE_QUOTA_BYTES，单文件上限不应超过总配额", "max", c.DriveMaxBytes, "quota", c.DriveQuotaBytes)
	}
	if _, ok := allowedLogLevels[c.LogLevel]; !ok {
		slog.Warn("LOG_LEVEL 非法，已回退为 info", "value", c.LogLevel)
	}
}

// validPort 校验端口是否为 1-65535 的十进制整数。
func validPort(p string) bool {
	n, err := strconv.Atoi(p)
	return err == nil && n >= 1 && n <= 65535
}

// validateCORSOrigins 要求每个非空来源都是合法的 http(s)://host 形式；畸形项无法被
// 中间件匹配，等于该来源被静默拒绝，因此生产环境直接 fail-fast。
func validateCORSOrigins(s string) error {
	for _, raw := range strings.Split(s, ",") {
		o := strings.TrimSpace(raw)
		if o == "" {
			continue
		}
		if !validOrigin(o) {
			return fmt.Errorf("CORS_ORIGINS 含非法来源 %q，应为 http(s)://host 形式", o)
		}
	}
	return nil
}

// warnCORSOrigins 与 validateCORSOrigins 同源，开发环境只告警。
func warnCORSOrigins(s string) {
	for _, raw := range strings.Split(s, ",") {
		o := strings.TrimSpace(raw)
		if o == "" {
			continue
		}
		if !validOrigin(o) {
			slog.Warn("CORS_ORIGINS 含非法来源，将被静默忽略", "origin", o)
		}
	}
}

// validOrigin 判断一个来源是否为合法的 http(s)://host 形式。
func validOrigin(o string) bool {
	u, err := url.Parse(o)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envOrBool 解析布尔型配置；空值或解析失败时回退默认值，与模块开关保持一致的宽容策略。
func envOrBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
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
		// 配置加载阶段 logger 尚未初始化，直接用默认 slog（输出到 stderr）。
		slog.Error("failed to generate JWT secret", "error", err)
		os.Exit(1)
	}
	return hex.EncodeToString(b)
}

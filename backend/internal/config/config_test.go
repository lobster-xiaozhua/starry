package config

import "testing"

// 生产环境必须配置数据库与 JWT 等关键信息，缺失应 fail-fast。
func TestValidate_ProductionRequiresSecrets(t *testing.T) {
	cfg := &Config{AppEnv: "production", DBHost: "h", DBUser: "u", DBName: "n"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("生产环境缺 DB_PASSWORD 应报错")
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("生产环境缺 JWT_SECRET 应报错")
	}

	cfg.DBPassword = "p"
	cfg.JWTSecret = "s"
	if err := cfg.Validate(); err == nil {
		t.Fatal("生产环境 CORS_ORIGINS 为空应报错")
	}

	cfg.CorsOrigins = "http://example.com"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("配置齐全时应通过，实际: %v", err)
	}
}

// 开发环境对弱配置仅告警，不阻断启动。
func TestValidate_DevelopmentWarnsOnly(t *testing.T) {
	cfg := &Config{AppEnv: "development"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("开发环境不应失败，实际: %v", err)
	}
}

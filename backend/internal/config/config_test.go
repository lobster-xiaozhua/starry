package config

import "testing"

// 生产环境必须配置数据库与 JWT 等关键信息，缺失应 fail-fast。
func TestValidate_ProductionRequiresSecrets(t *testing.T) {
	cfg := &Config{
		AppEnv:          "production",
		Port:            "8080",
		MaxBodyBytes:    8 << 20,
		DriveMaxBytes:   50 << 20,
		DriveQuotaBytes: 1 << 30,
		DBHost:          "h", DBUser: "u", DBName: "n",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("生产环境缺 DB_PASSWORD 应报错")
	}
	cfg.DBPassword = "p"
	if err := cfg.Validate(); err == nil {
		t.Fatal("生产环境缺 JWT_SECRET 应报错")
	}
	cfg.JWTSecret = "s"
	if err := cfg.Validate(); err == nil {
		t.Fatal("生产环境 CORS_ORIGINS 为空应报错")
	}

	cfg.CorsOrigins = "http://example.com"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("配置齐全时应通过，实际: %v", err)
	}
}

// 开发环境对弱配置仅告警，不阻断启动（即便 PORT/MAX_BODY_BYTES 缺省也允许 boot）。
func TestValidate_DevelopmentWarnsOnly(t *testing.T) {
	cfg := &Config{AppEnv: "development"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("开发环境不应失败，实际: %v", err)
	}
}

// 数值型 sanity 在生产环境必须 fail-fast：畸形端口、过小 body 上限、单文件上限超过总配额。
func TestValidate_ProductionRejectsBadNumericConfig(t *testing.T) {
	base := &Config{
		AppEnv: "production",
		DBHost: "h", DBUser: "u", DBName: "n", DBPassword: "p", JWTSecret: "s",
		CorsOrigins: "http://example.com",
	}
	cases := []struct {
		name string
		mut  func(c *Config)
	}{
		{"empty port", func(c *Config) { c.Port = "" }},
		{"non-numeric port", func(c *Config) { c.Port = "abc" }},
		{"out-of-range port", func(c *Config) { c.Port = "70000" }},
		{"zero body", func(c *Config) { c.Port = "8080"; c.MaxBodyBytes = 0 }},
		{"tiny body", func(c *Config) { c.Port = "8080"; c.MaxBodyBytes = 100 }},
		{"drive max exceeds quota", func(c *Config) {
			c.Port = "8080"
			c.MaxBodyBytes = 8 << 20
			c.DriveMaxBytes = 2 << 30
			c.DriveQuotaBytes = 1 << 30
		}},
	}
	for _, tc := range cases {
		c := *base
		c.MaxBodyBytes = 8 << 20
		c.DriveMaxBytes = 50 << 20
		c.DriveQuotaBytes = 1 << 30
		tc.mut(&c)
		if err := c.Validate(); err == nil {
			t.Fatalf("case %q 应被生产校验拒绝", tc.name)
		}
	}
}

// 合法数值配置在生产环境应通过。
func TestValidate_ProductionAcceptsValidNumericConfig(t *testing.T) {
	cfg := &Config{
		AppEnv:          "production",
		Port:            "8080",
		MaxBodyBytes:    8 << 20,
		DriveMaxBytes:   50 << 20,
		DriveQuotaBytes: 1 << 30,
		LogLevel:        "info",
		DBHost:          "h", DBUser: "u", DBName: "n", DBPassword: "p", JWTSecret: "s",
		CorsOrigins: "http://example.com, https://app.example.com",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("合法配置应通过，实际: %v", err)
	}
}

// CORS 来源格式校验：合法 http(s)://host 通过，缺 scheme / 空项 / 畸形项被拒（生产）。
func TestValidate_CORSOriginFormat(t *testing.T) {
	good := []string{
		"http://localhost:5173",
		"https://app.example.com",
		"http://a.com, https://b.com", // 逗号分隔、含空格、多来源
	}
	for _, s := range good {
		if err := validateCORSOrigins(s); err != nil {
			t.Fatalf("合法来源 %q 不应报错: %v", s, err)
		}
	}
	bad := []string{
		"localhost:5173",          // 缺 scheme
		"ftp://files.example.com", // 非 http(s)
		"http://",                 // 缺 host
		"not a url",
	}
	for _, s := range bad {
		if err := validateCORSOrigins(s); err == nil {
			t.Fatalf("非法来源 %q 应报错", s)
		}
	}
}

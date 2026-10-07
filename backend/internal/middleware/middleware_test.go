package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/authpkg"
)

func newEngine(mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mw...)
	r.GET("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func get(r *gin.Engine, remoteAddr string, headers map[string]string, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/probe"+query, nil)
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestRateLimitBlocksAfterMax 验证限流在窗口内放行 max 次、第 max+1 次返回 429。
// 该中间件保护 /auth/captcha 与 /auth/refresh 两个可被滥用的匿名端点。
func TestRateLimitBlocksAfterMax(t *testing.T) {
	r := newEngine(RateLimit(3, time.Minute))
	const ip = "203.0.113.9:5555"

	for i := 1; i <= 3; i++ {
		if w := get(r, ip, nil, ""); w.Code != http.StatusOK {
			t.Fatalf("第 %d 次请求应放行，实际 %d", i, w.Code)
		}
	}
	if w := get(r, ip, nil, ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("第 4 次请求应被限流(429)，实际 %d", w.Code)
	}
	// 另一个来源 IP 独立计数，不应被前一个 IP 限流影响。
	if w := get(r, "198.51.100.7:5555", nil, ""); w.Code != http.StatusOK {
		t.Fatalf("不同 IP 应独立计数，实际 %d", w.Code)
	}
}

// TestJWTAuth 覆盖令牌缺失、无效与有效三种路径，并支持 query 传参（SSE 无法带 header）。
func TestJWTAuth(t *testing.T) {
	const secret = "test-secret"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(JWTAuth(secret))
	r.GET("/probe", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"userID": c.GetString("userID"), "role": c.GetString("role")})
	})

	if w := get(r, "", nil, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("缺少令牌应 401，实际 %d", w.Code)
	}
	if w := get(r, "", map[string]string{"Authorization": "Bearer not-a-token"}, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("无效令牌应 401，实际 %d", w.Code)
	}

	token, _, err := authpkg.IssueAccessToken(secret, "user-123", "admin", time.Minute)
	if err != nil {
		t.Fatalf("签发令牌失败: %v", err)
	}
	// header 传参
	w := get(r, "", map[string]string{"Authorization": "Bearer " + token}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("有效令牌应放行，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !contains(w.Body.String(), "user-123") || !contains(w.Body.String(), "admin") {
		t.Fatalf("应把 userID/role 注入上下文，实际 %s", w.Body.String())
	}
	// query 传参（SSE 通道）
	if w := get(r, "", nil, "?token="+token); w.Code != http.StatusOK {
		t.Fatalf("query 传参的有效令牌应放行，实际 %d", w.Code)
	}
}

// TestRequireUserAndAdmin 校验角色门禁语义：user 不能进管理端，admin 两者皆可，
// 无角色（未过 JWTAuth）一律拒绝。
func TestRequireUserAndAdmin(t *testing.T) {
	cases := []struct {
		role       string
		mw         gin.HandlerFunc
		wantStatus int
		name       string
	}{
		{"user", RequireUser(), http.StatusOK, "普通用户可访问用户级接口"},
		{"user", RequireAdmin(), http.StatusForbidden, "普通用户不可访问管理端"},
		{"admin", RequireUser(), http.StatusOK, "管理员可访问用户级接口"},
		{"admin", RequireAdmin(), http.StatusOK, "管理员可访问管理端"},
		{"", RequireUser(), http.StatusForbidden, "无角色应拒绝"},
		{"", RequireAdmin(), http.StatusForbidden, "无角色应拒绝管理端"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newEngine(func(c *gin.Context) { c.Set("role", tc.role); c.Next() }, tc.mw)
			if w := get(r, "", nil, ""); w.Code != tc.wantStatus {
				t.Fatalf("role=%q 期望 %d，实际 %d", tc.role, tc.wantStatus, w.Code)
			}
		})
	}
}

// TestCORSWhitelist 校验来源白名单：白名单内回写 CORS 头，白名单外不回写
// （避免任意站点携带凭据访问），且 allowed="*" 时不回写通配。
func TestCORSWhitelist(t *testing.T) {
	r := newEngine(CORS("https://app.example.com, https://admin.example.com"))

	w := get(r, "", map[string]string{"Origin": "https://app.example.com"}, "")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("白名单来源应回写 CORS 头，实际 %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("应允许凭据，实际 %q", got)
	}

	w = get(r, "", map[string]string{"Origin": "https://evil.example.com"}, "")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("非白名单来源不应回写 CORS 头，实际 %q", got)
	}

	// 显式配置 "*" 时同样不回写（防止凭据型跨站泄露）
	r2 := newEngine(CORS("*"))
	w = get(r2, "", map[string]string{"Origin": "https://any.example.com"}, "")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("配置 '*' 时不应回写通配来源，实际 %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

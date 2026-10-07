package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/logx"
)

// captureLogs 把全局 logger 接到内存 buffer 上，返回读取函数与还原函数。
func captureLogs(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prev := logx.L()
	logx.SetLogger(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return &buf, func() { logx.SetLogger(prev) }
}

func TestRequestIDPropagatesAndIsGenerated(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, RequestIDFrom(c))
	})

	// 上游已注入时沿用，便于跨服务串联链路。
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-ID", "upstream-123")
	r.ServeHTTP(w, req)
	if w.Body.String() != "upstream-123" {
		t.Fatalf("应沿用上游 request id, got %q", w.Body.String())
	}
	if w.Header().Get("X-Request-ID") != "upstream-123" {
		t.Fatalf("响应头应回写 request id, got %q", w.Header().Get("X-Request-ID"))
	}

	// 未注入时生成，且每次不同。
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	generated := w.Body.String()
	if generated == "" || generated == "-" {
		t.Fatalf("应生成 request id, got %q", generated)
	}
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if w2.Body.String() == generated {
		t.Fatalf("两次请求的 request id 不应相同: %q", generated)
	}
}

// TestRequestLoggerRedactsTokenInQuery 是凭据泄露回归测试：
// SSE 端点通过 query 传 token，访问日志必须脱敏，否则凭据会随日志长期落盘。
func TestRequestLoggerRedactsTokenInQuery(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	r := gin.New()
	r.Use(RequestID(), RequestLogger())
	r.GET("/api/chat/stream", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/chat/stream?token=super-secret-jwt&session=s1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("日志中间件不应影响响应, got %d", w.Code)
	}
	out := buf.String()
	if strings.Contains(out, "super-secret-jwt") {
		t.Fatalf("访问日志泄露了 token: %s", out)
	}
	if !strings.Contains(out, "%2A%2A%2A") {
		t.Fatalf("访问日志应出现脱敏占位符: %s", out)
	}
	if !strings.Contains(out, "session=s1") {
		t.Fatalf("非敏感参数应保留: %s", out)
	}
}

func TestRequestLoggerSkipsHealthButLogsOthers(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	r := gin.New()
	r.Use(RequestID(), RequestLogger("/health"))
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/slow", func(c *gin.Context) { c.Status(http.StatusOK) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	if buf.Len() != 0 {
		t.Fatalf("健康检查不应写访问日志（探活会淹没日志）: %s", buf.String())
	}
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/slow", nil))
	if !strings.Contains(buf.String(), `"path":"/slow"`) {
		t.Fatalf("应记录普通请求: %s", buf.String())
	}
}

func TestSanitizePathRedactsCredentials(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"/api/chat/stream", "/api/chat/stream"},
		{"/api/chat/stream?token=abc", "/api/chat/stream?token=%2A%2A%2A"},
		{"/x?password=p&a=1", "/x?a=1&password=%2A%2A%2A"},
		{"/x?code=c&refresh_token=r", "/x?code=%2A%2A%2A&refresh_token=%2A%2A%2A"},
	}
	for _, c := range cases {
		u, err := url.Parse(c.raw)
		if err != nil {
			t.Fatalf("parse %q: %v", c.raw, err)
		}
		if got := sanitizePath(u); got != c.want {
			t.Fatalf("sanitizePath(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
	if got := sanitizePath(nil); got != "" {
		t.Fatalf("nil URL 应返回空串, got %q", got)
	}
}

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
)

func TestRecovery_PanicReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/boom", func(c *gin.Context) {
		panic("kaboom")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法信封: %v (body=%s)", err, w.Body.String())
	}
	if body.Code != 5000 || body.Message == "" {
		t.Fatalf("信封内容不符合约定: %+v", body)
	}
}

func TestRecovery_PassesThroughNormalRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/ok", func(c *gin.Context) {
		c.String(http.StatusOK, "fine")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK || w.Body.String() != "fine" {
		t.Fatalf("正常请求应透传，got %d %q", w.Code, w.Body.String())
	}
}

// TestCoreFail_AbortsToPreventDoubleWrite 锁定一个真实回归：gin 的引擎在调用 Next()
// 的 for 循环里逐个执行 handler，只有 Abort() 能中断；中间件拒绝后若不 Abort，链会继续
// 落到业务 handler 再写一次 200，响应体被拼接成多段 JSON、状态码错乱。core.Fail 必须在写
// 完响应后 Abort。
func TestCoreFail_AbortsToPreventDoubleWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/x",
		func(c *gin.Context) {
			core.Fail(c, http.StatusForbidden, 1006, "denied")
			// 不调用 c.Next()：模拟鉴权中间件拒绝后直接返回
		},
		func(c *gin.Context) {
			core.OK(c, gin.H{"ok": true}) // 若 core.Fail 未 Abort，这里会二次写入
		},
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if w.Body.String() != `{"code":1006,"message":"denied","data":null}` {
		t.Fatalf("应只写一次拒绝信封，实际: %q", w.Body.String())
	}
}

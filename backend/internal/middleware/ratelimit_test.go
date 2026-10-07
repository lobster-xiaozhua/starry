package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// stubCounter 记录调用并可控失败，用来验证限流策略而不依赖真实 Redis。
type stubCounter struct {
	mu     sync.Mutex
	keys   []string
	fail   bool
	counts map[string]int64
}

func (s *stubCounter) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = append(s.keys, key)
	if s.fail {
		return 0, errors.New("redis down")
	}
	s.counts[key]++
	return s.counts[key], nil
}

func newStub() *stubCounter { return &stubCounter{counts: map[string]int64{}} }

func doGet(r *gin.Engine, ip string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = ip + ":1234"
	r.ServeHTTP(w, req)
	return w
}

// TestLimiterBlocksAfterMaxAndScopesByIP 验证额度、作用域与 IP 维度。
func TestLimiterBlocksAfterMaxAndScopesByIP(t *testing.T) {
	stub := newStub()
	r := gin.New()
	r.Use(NewLimiter(stub).Limit("auth:captcha", 2, time.Minute))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 2; i++ {
		if w := doGet(r, "10.0.0.1"); w.Code != http.StatusOK {
			t.Fatalf("第 %d 次应放行, got %d", i+1, w.Code)
		}
	}
	if w := doGet(r, "10.0.0.1"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("第 3 次应限流, got %d", w.Code)
	}
	// 另一个 IP 独立计额。
	if w := doGet(r, "10.0.0.2"); w.Code != http.StatusOK {
		t.Fatalf("不同 IP 应独立计额, got %d", w.Code)
	}
	for _, k := range stub.keys {
		if !strings.HasPrefix(k, "auth:captcha:") {
			t.Fatalf("计数 key 应带作用域前缀, got %q", k)
		}
	}
}

// TestLimiterFailsOpenOnCounterError 验证计数不可用时限流放行而不是全站拒绝：
// 因限流组件故障导致业务整体不可用，比短时不限流的损失大得多。
func TestLimiterFailsOpenOnCounterError(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	stub := newStub()
	stub.fail = true
	r := gin.New()
	r.Use(NewLimiter(stub).Limit("auth:refresh", 1, time.Minute))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 3; i++ {
		if w := doGet(r, "10.0.0.1"); w.Code != http.StatusOK {
			t.Fatalf("计数失败时应 fail-open, got %d", w.Code)
		}
	}
	if !strings.Contains(buf.String(), "fail-open") {
		t.Fatalf("fail-open 应留下告警日志: %s", buf.String())
	}
}

func TestLimiterZeroMaxIsNoop(t *testing.T) {
	stub := newStub()
	r := gin.New()
	r.Use(NewLimiter(stub).Limit("x", 0, time.Minute))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	if w := doGet(r, "10.0.0.1"); w.Code != http.StatusOK {
		t.Fatalf("max<=0 应直接放行, got %d", w.Code)
	}
	if len(stub.keys) != 0 {
		t.Fatalf("max<=0 不应产生计数调用, got %v", stub.keys)
	}
}

// TestNilLimiterNeverPanics 保证模块在组合根漏注入 Limiter 时也不会 panic。
func TestNilLimiterNeverPanics(t *testing.T) {
	var l *Limiter
	r := gin.New()
	r.Use(l.Limit("x", 1, time.Minute))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	if w := doGet(r, "10.0.0.1"); w.Code != http.StatusOK {
		t.Fatalf("nil limiter 应放行, got %d", w.Code)
	}
}

// TestMemoryCounterResetsAfterWindow 验证进程内计数会随窗口结束重置，
// 否则一次限流会把客户端永久封死。
func TestMemoryCounterResetsAfterWindow(t *testing.T) {
	c := NewMemoryCounter()
	ctx := context.Background()
	for i := int64(1); i <= 3; i++ {
		got, err := c.Incr(ctx, "k", 20*time.Millisecond)
		if err != nil {
			t.Fatalf("incr: %v", err)
		}
		if got != i {
			t.Fatalf("第 %d 次计数 = %d", i, got)
		}
	}
	time.Sleep(40 * time.Millisecond)
	got, err := c.Incr(ctx, "k", 20*time.Millisecond)
	if err != nil {
		t.Fatalf("incr: %v", err)
	}
	if got != 1 {
		t.Fatalf("窗口结束后应重新计数, got %d", got)
	}
}

func TestMemoryCounterSweepDropsExpired(t *testing.T) {
	c := NewMemoryCounter()
	_, _ = c.Incr(context.Background(), "old", 10*time.Millisecond)
	_, _ = c.Incr(context.Background(), "keep", 10*time.Minute)
	time.Sleep(20 * time.Millisecond)
	c.sweep(time.Now())
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.buckets["old"]; ok {
		t.Fatalf("过期 bucket 应被清扫")
	}
	if _, ok := c.buckets["keep"]; !ok {
		t.Fatalf("未过期 bucket 不应被清扫")
	}
}

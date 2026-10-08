package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// bodyEngine 构造一个带体积上限的测试引擎：handler 原样读回请求体，
// 用于观察「限制是否真的作用到了读取环节」而不是被上层魔法吞掉。
func bodyEngine(limit int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(limit))
	r.POST("/echo", func(c *gin.Context) {
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			FailBodyTooLarge(c, limit)
			return
		}
		c.JSON(http.StatusOK, gin.H{"len": len(b)})
	})
	// /silent 的 handler 读完就返回，既不判错也不写响应，用来验证中间件的兜底分支。
	r.POST("/silent", func(c *gin.Context) {
		_, _ = io.ReadAll(c.Request.Body)
	})
	return r
}

func post(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestBodyLimit 覆盖三种体积关系：远低于上限、恰好在上限内、超出上限。
// 「恰好在上限内」这条边界最容易写错——少一个字节的差池就会让合法请求失败。
func TestBodyLimit(t *testing.T) {
	const limit = 1024
	cases := []struct {
		name     string
		bodyLen  int
		wantCode int
	}{
		{"远低于上限", 16, http.StatusOK},
		{"恰好等于上限", limit, http.StatusOK},
		{"超出上限一个字节", limit + 1, http.StatusRequestEntityTooLarge},
		{"远超上限", limit * 4, http.StatusRequestEntityTooLarge},
	}
	r := bodyEngine(limit)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := post(r, "/echo", strings.Repeat("x", tc.bodyLen))
			if w.Code != tc.wantCode {
				t.Fatalf("期望状态码 %d，实际 %d（body=%d 字节）", tc.wantCode, w.Code, tc.bodyLen)
			}
		})
	}
}

// TestBodyLimitBehindJSONBinder 记录一个已知的取舍：走 c.ShouldBindJSON 的普通接口，
// 超限会被绑定器先一步翻译成 400——它读不到「这是超限」的语义。
// 中间件无法在 handler 之前独占状态码（见 BodyLimit 注释），因此这是预期行为：
// 资源已受限于 MAX_BODY_BYTES，只是状态码不够精确。
// 真正需要精确 413 的入口应当像 notes.Import 那样自行判定 IsBodyTooLarge。
func TestBodyLimitBehindJSONBinder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(64))
	r.POST("/bind", func(c *gin.Context) {
		var payload map[string]any
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusOK)
	})

	// 必须是「语法合法但超长」的 JSON：先用非法字符测的话，绑定器会在撞上限之前
	// 就因为语法错误返回，测到的就不是超限行为了。
	payload := `{"title":"` + strings.Repeat("x", 1024) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/bind", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("绑定器型接口在超限时返回 400（已知行为），实际 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "too large") {
		t.Fatalf("响应应透出超限原因，实际: %s", w.Body.String())
	}
}

// TestDeclaredEntryReports413 是配套的正面案例：入口自己判定 IsBodyTooLarge 时，
// 客户端就能拿到可据此重试/拆分的 413。notes 导入与网盘上传走的正是这条路。
func TestDeclaredEntryReports413(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const entryLimit = 1024
	r := gin.New()
	r.Use(BodyLimit(64))
	r.POST("/import", func(c *gin.Context) {
		RaiseBodyLimit(c, entryLimit)
		if _, err := io.ReadAll(c.Request.Body); IsBodyTooLarge(err) {
			FailBodyTooLarge(c, entryLimit)
			return
		}
		c.Status(http.StatusOK)
	})

	cases := []struct {
		name string
		size int
		want int
	}{
		{"低于声明额度", 512, http.StatusOK},
		{"高于声明额度", 4096, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/import", strings.NewReader(strings.Repeat("x", tc.size)))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("期望 %d，实际 %d", tc.want, w.Code)
			}
		})
	}
}

// TestBodyLimitFallsBackWhenHandlerSilent 验证 chunked（无 Content-Length）场景下，
// handler 吞掉超限错误时中间件仍给出 413。少了这条兜底，客户端只会看到空响应，
// 前端只能显示一个不明所以的网络错误。
func TestBodyLimitFallsBackWhenHandlerSilent(t *testing.T) {
	r := bodyEngine(64)
	req := httptest.NewRequest(http.MethodPost, "/silent", strings.NewReader(strings.Repeat("x", 256)))
	req.ContentLength = -1 // 模拟 chunked：Content-Length 未知，只能靠读取时发现超限
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("期望兜底返回 413，实际 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "code") {
		t.Fatalf("兜底响应应沿用统一信封，实际: %s", w.Body.String())
	}
}

// TestRaiseBodyLimit 验证 handler 可以把额度抬高到自己需要的档位——
// 这是上传类接口唯一正当的扩容口子，抬不到就等于功能不可用。
func TestRaiseBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(64))
	r.POST("/upload", func(c *gin.Context) {
		if !RaiseBodyLimit(c, 4096) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "raise failed"})
			return
		}
		b, err := io.ReadAll(c.Request.Body)
		if IsBodyTooLarge(err) {
			FailBodyTooLarge(c, 4096)
			return
		}
		c.JSON(http.StatusOK, gin.H{"len": len(b)})
	})

	w := post(r, "/upload", strings.Repeat("x", 2048))
	if w.Code != http.StatusOK {
		t.Fatalf("抬高额度后应放行 2048 字节，实际 %d", w.Code)
	}
	// 抬高之后上限依然生效，而不是被无上限替换掉。
	w = post(r, "/upload", strings.Repeat("x", 8192))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("抬高后仍应受新上限约束，实际 %d", w.Code)
	}
}

// TestRaiseBodyLimitWithoutInstalledMiddleware 验证没有 BodyLimit 时 Raise 返回 false，
// 调用方据此可以降级处理，而不是误以为额度已经生效。
func TestRaiseBodyLimitWithoutInstalledMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("x"))
	if RaiseBodyLimit(c, 4096) {
		t.Fatal("未安装 BodyLimit 时 RaiseBodyLimit 应返回 false")
	}
}

// TestBodyLimitDisabledOnNonPositive 是防御性回归：0 或负值代表「不限制」，
// 这是运维误配或未读默认值时的常见取值，不能退化成「连 1 字节都读不了」。
func TestBodyLimitDisabledOnNonPositive(t *testing.T) {
	for _, limit := range []int64{0, -1} {
		r := bodyEngine(limit)
		w := post(r, "/echo", strings.Repeat("x", 4096))
		if w.Code != http.StatusOK {
			t.Fatalf("limit=%d 应代表不限，实际状态码 %d", limit, w.Code)
		}
	}
}

// TestIsBodyTooLarge 是关键的一环：超限判定必须走 errors.As 匹配 *http.MaxBytesError，
// 而不是匹配错误文案。标准库改一个标点就让字符串匹配静默失效，届时超限请求会被
// 当成普通 400 处理，客户端永远拿不到「太大了」这个可据此重试或拆分的信号。
func TestIsBodyTooLarge(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 8)))
	limited := http.MaxBytesReader(httptest.NewRecorder(), req.Body, 2)
	_, tooLargeErr := io.ReadAll(limited)
	if tooLargeErr == nil {
		t.Fatal("构造 MaxBytesError 失败：ReadAll 未返回错误")
	}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"普通错误", io.ErrUnexpectedEOF, false},
		{"超限错误", tooLargeErr, true},
		{"被包装的超限错误", fmt.Errorf("read failed: %w", tooLargeErr), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsBodyTooLarge(tc.err); got != tc.want {
				t.Fatalf("IsBodyTooLarge = %v，期望 %v", got, tc.want)
			}
		})
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1 KiB"},
		{8 << 20, "8 MiB"},
		{3 << 30, "3 GiB"},
	}
	for _, tc := range cases {
		if got := HumanBytes(tc.in); got != tc.want {
			t.Fatalf("HumanBytes(%d) = %q，期望 %q", tc.in, got, tc.want)
		}
	}
}

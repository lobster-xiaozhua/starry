package middleware

import (
	"net/http"
	"testing"
)

// TestSecurityHeadersAlwaysApplied 校验无论部署形态如何都会下发的基线防护头。
// 这些都是「少配一个就少一层」的防线，测试的价值在于防止有人在重构中间件链时
// 顺手把整段删掉而无人察觉。
func TestSecurityHeadersAlwaysApplied(t *testing.T) {
	cases := []struct {
		name string
		hsts bool
	}{
		{"开发环境不启用 HSTS", false},
		{"生产环境启用 HSTS", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newEngine(SecurityHeaders(tc.hsts))
			w := get(r, "203.0.113.1:1234", nil, "")

			want := map[string]string{
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
				"Referrer-Policy":        "no-referrer",
				"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
			}
			for k, v := range want {
				if got := w.Header().Get(k); got != v {
					t.Errorf("%s 期望 %q，实际 %q", k, v, got)
				}
			}

			hsts := w.Header().Get("Strict-Transport-Security")
			if tc.hsts && hsts == "" {
				t.Error("启用 HSTS 时未下发 Strict-Transport-Security")
			}
			if !tc.hsts && hsts != "" {
				t.Errorf("未启用 HSTS 时不应下发 Strict-Transport-Security，实际 %q", hsts)
			}
		})
	}
}

// TestSecurityHeadersDoNotBreakNormalResponse 防止「加响应头」顺手改写了状态码或响应体——
// 中间件最容易犯的错就是在 c.Next() 之前就把请求终止掉。
func TestSecurityHeadersDoNotBreakNormalResponse(t *testing.T) {
	r := newEngine(SecurityHeaders(true))
	w := get(r, "203.0.113.1:1234", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应保持 200，实际 %d", w.Code)
	}
}

package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/logx"
)

// HeaderRequestID 是贯穿一次请求的追踪标识；若上游（网关/Nginx）已注入则沿用，
// 否则由本服务生成并回写，便于把前端报错与服务端日志对齐。
const HeaderRequestID = "X-Request-ID"

// requestIDKey 是 gin context 内的键名，供业务 handler 写入日志。
const requestIDKey = "requestID"

// RequestID 为每次请求注入追踪 ID。放在中间件链最前，后续中间件与 handler 均可读取。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = newRequestID()
		}
		c.Set(requestIDKey, id)
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

// RequestIDFrom 取出当前请求的追踪 ID，缺失时返回 "-"。
func RequestIDFrom(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "-"
}

// RequestLogger 输出结构化访问日志。
//
// 注意：SSE 等端点通过 query 传递 token，访问日志会把 query 中的敏感参数整体脱敏，
// 避免凭据落盘到日志系统——这是常见的线上泄密点。
func RequestLogger(skipPaths ...string) gin.HandlerFunc {
	skip := map[string]bool{}
	for _, p := range skipPaths {
		skip[p] = true
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		if skip[c.FullPath()] {
			return
		}
		latency := time.Since(start)
		status := c.Writer.Status()
		attrs := []any{
			slog.String("request_id", RequestIDFrom(c)),
			slog.String("method", c.Request.Method),
			slog.String("path", sanitizePath(c.Request.URL)),
			slog.Int("status", status),
			slog.Int64("latency_ms", latency.Milliseconds()),
			slog.String("ip", c.ClientIP()),
			slog.Int("bytes", c.Writer.Size()),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
		}
		// 流式响应没有 WriteTimeout 保护，慢请求额外用 warn 标出便于排查连接堆积。
		if latency > 10*time.Second {
			attrs = append(attrs, slog.Bool("slow", true))
			logx.L().Warn("slow request", attrs...)
			return
		}
		switch {
		case status >= http.StatusInternalServerError:
			logx.L().Error("request failed", attrs...)
		case status >= http.StatusBadRequest:
			logx.L().Warn("request rejected", attrs...)
		default:
			logx.L().Debug("request", attrs...)
		}
	}
}

// sanitizePath 对 URL query 中的凭据类参数做脱敏。
func sanitizePath(u *url.URL) string {
	if u == nil {
		return ""
	}
	path := u.Path
	if u.RawQuery == "" {
		return path
	}
	q := u.Query()
	for _, k := range []string{"token", "access_token", "refresh_token", "code", "password", "secret"} {
		if _, ok := q[k]; ok {
			q.Set(k, "***")
		}
	}
	return path + "?" + q.Encode()
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 随机数不可用时退化为时间戳，保证日志链路不断。
		return "rid-" + time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}

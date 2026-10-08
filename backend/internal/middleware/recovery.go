package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"log/slog"

	"starry/backend/internal/core"
	"starry/backend/internal/logx"
)

// Recovery 自定义 panic 恢复中间件，取代 gin.Recovery()：
//   - 以统一的 core 信封返回 500（默认的 gin.Recovery 返回纯文本，与全站响应契约不一致，
//     下游（尤其前端）只能按状态码分支，丢失了 message 等可读信息）；
//   - 用结构化日志记录 panic 值与堆栈，便于日志管线聚合与排障；
//   - 已写出响应的 panic 不二次写入，避免破坏半成功的流式/SSE 响应。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logx.L().Error("panic recovered",
					slog.Any("error", r),
					slog.String("path", c.Request.URL.Path),
					slog.String("method", c.Request.Method),
					slog.String("request_id", RequestIDFrom(c)),
					slog.String("stack", string(debug.Stack())),
				)
				if !c.Writer.Written() {
					core.Fail(c, http.StatusInternalServerError, 5000, "服务器内部错误")
				}
			}
		}()
		c.Next()
	}
}

// Package core 是被各业务模块共享的「内核」：统一 HTTP 响应契约与共享依赖容器。
// 业务模块（internal/modules/*）只依赖本包与其他基础设施包，彼此不互相依赖，
// 以此保证模块边界清晰、可独立装配与开关。
package core

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 是全局统一的 API 响应信封。
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// OK 返回业务成功响应（HTTP 200，code=0）。
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

// Fail 返回业务错误响应，httpStatus 为 HTTP 状态码，code 为业务码。
// 写入后即 Abort：错误/拒绝响应必须是终态，否则 gin 链会继续执行下一个
// handler（例如鉴权中间件拒绝后若不断开，请求会落到业务 handler 又写一次 200），
// 造成同一请求被多次写响应（响应体被拼接、状态码错乱）。
func Fail(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, Response{Code: code, Message: message, Data: nil})
	c.Abort()
}

// FailWithField 返回带字段级错误明细的响应，用于表单校验失败。
func FailWithField(c *gin.Context, httpStatus, code int, message string, fields map[string]string) {
	c.JSON(httpStatus, Response{Code: code, Message: message, Data: gin.H{"fields": fields}})
	c.Abort()
}

// ClientIP 返回客户端 IP，便于各模块统一取用（审计、限流键）。
func ClientIP(c *gin.Context) string {
	return c.ClientIP()
}

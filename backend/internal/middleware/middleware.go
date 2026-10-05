package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"login-system/backend/internal/authpkg"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-Id")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// JWTAuth 从 Authorization header 解析访问令牌；SSE 等无法带 header 的通道
// 通过 query 参数 token 传参，middleware 兼容两种来源。
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := extractClaims(c, secret)
		if err != nil {
			abort401(c)
			return
		}
		c.Set("userID", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func extractClaims(c *gin.Context, secret string) (*authpkg.Claims, error) {
	header := c.GetHeader("Authorization")
	if header != "" && strings.HasPrefix(header, "Bearer ") {
		return authpkg.ParseAccessToken(secret, strings.TrimPrefix(header, "Bearer "))
	}
	if q := c.Query("token"); q != "" {
		return authpkg.ParseAccessToken(secret, q)
	}
	return nil, errors.New("missing token")
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    1006,
				"message": "需要管理员权限",
				"data":    nil,
			})
			return
		}
		c.Next()
	}
}

// RequireNotes 云笔记模块权限：role ∈ {user, admin}。
func RequireNotes() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role != "user" && role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    1006,
				"message": "无云笔记访问权限",
				"data":    nil,
			})
			return
		}
		c.Next()
	}
}

func abort401(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"code":    1005,
		"message": "登录状态无效或已过期",
		"data":    nil,
	})
}

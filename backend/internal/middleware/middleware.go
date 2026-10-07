package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/authpkg"
)

// CORS 按白名单校验请求来源。allowed 为空或 "*" 时退化为同源策略：
// 不回写任意 origin，避免凭据型跨站泄露。多来源以逗号分隔传入。
func CORS(allowed string) gin.HandlerFunc {
	allowedSet := map[string]bool{}
	if allowed != "" && allowed != "*" {
		for _, o := range strings.Split(allowed, ",") {
			if t := strings.TrimSpace(o); t != "" {
				allowedSet[t] = true
			}
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowedSet[origin] {
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

// RequireUser 已登录用户权限：role ∈ {user, admin} 即可访问。
// 原名为 RequireNotes（仅用于云笔记模块），实际语义覆盖全部已登录用户，故更名。
func RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role != "user" && role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    1006,
				"message": "需要登录后访问",
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

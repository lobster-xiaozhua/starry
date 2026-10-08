// Package auth 认证模块：注册登录、注册、找回/重置密码、令牌刷新与验证码等端点。
// 模块自持 AuthService，仅从 core.Deps 取用基础设施，不依赖其他业务模块。
package auth

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/middleware"
)

// Register 装配并挂载认证模块路由。
func Register(api *gin.RouterGroup, d *core.Deps) {
	svc := NewService(d.DB, d.Redis, d.Cfg.JWTSecret)
	// 验证码校验器依赖生成器实例，在此完成闭环，避免组合根反向依赖本模块细节。
	svc.SetCaptchaVerifier(func(_ context.Context, captchaID, answer string) bool {
		return d.Captcha.Verify(captchaID, answer, true)
	})
	h := New(svc, d.Settings, d.Captcha)

	g := api.Group("/auth")
	{
		// 匿名可访问的写端点全部限流：额度由组合根注入的 Limiter 决定
		// （多实例部署时计数共享于 Redis，不会随副本数放大）。
		// 限流只是第一层，账号维度的防暴破仍由失败计数锁定兜底。
		g.POST("/captcha", d.Limiter.Limit("auth:captcha", 20, time.Minute), h.Captcha)
		g.POST("/login", d.Limiter.Limit("auth:login", 20, time.Minute), h.Login)
		g.POST("/register", d.Limiter.Limit("auth:register", 10, time.Minute), h.Register)
		g.GET("/password-policy", h.PasswordPolicy)
		g.POST("/forgot-password", d.Limiter.Limit("auth:forgot", 10, time.Minute), h.ForgotPassword)
		g.POST("/reset-password", h.ResetPassword)
		g.POST("/refresh", d.Limiter.Limit("auth:refresh", 30, time.Minute), h.Refresh)
		g.POST("/logout", middleware.JWTAuth(d.Cfg.JWTSecret), h.Logout)
		g.GET("/me", middleware.JWTAuth(d.Cfg.JWTSecret), h.Me)
		// 只读安全可观测端点：管理员专属，列出当前被锁定的账号。
		g.GET("/security/lockouts", middleware.JWTAuth(d.Cfg.JWTSecret), middleware.RequireAdmin(), h.ListLockouts)
	}
}

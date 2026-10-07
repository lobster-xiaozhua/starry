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
		// 验证码与刷新令牌是可被滥用的匿名端点，单独限流。
		g.POST("/captcha", middleware.RateLimit(20, time.Minute), h.Captcha)
		g.POST("/login", h.Login)
		g.POST("/register", h.Register)
		g.GET("/password-policy", h.PasswordPolicy)
		g.POST("/forgot-password", h.ForgotPassword)
		g.POST("/reset-password", h.ResetPassword)
		g.POST("/refresh", middleware.RateLimit(30, time.Minute), h.Refresh)
		g.POST("/logout", middleware.JWTAuth(d.Cfg.JWTSecret), h.Logout)
		g.GET("/me", middleware.JWTAuth(d.Cfg.JWTSecret), h.Me)
	}
}

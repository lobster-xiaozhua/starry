package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"

	"login-system/backend/internal/service"
	"login-system/backend/internal/store"
)

type AuthHandler struct {
	auth           *service.AuthService
	settingsSvc    *service.SettingsService
	captchaGen     *base64Captcha.Captcha
	captchaEnabled func(ctx context.Context) bool
}

func NewAuthHandler(auth *service.AuthService, settingsSvc *service.SettingsService, captchaGen *base64Captcha.Captcha) *AuthHandler {
	h := &AuthHandler{auth: auth, settingsSvc: settingsSvc, captchaGen: captchaGen}
	h.captchaEnabled = func(ctx context.Context) bool {
		settings, err := settingsSvc.Get(ctx)
		if err != nil {
			return false
		}
		return settings.CaptchaEnabled
	}
	return h
}

type loginRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	CaptchaID   string `json:"captchaId"`
	CaptchaCode string `json:"captchaCode"`
}

type registerRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type forgotRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type resetRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (h *AuthHandler) Captcha(c *gin.Context) {
	enabled := h.captchaEnabled(c.Request.Context())
	if !enabled {
		OK(c, gin.H{"enabled": false})
		return
	}
	id, b64, answer, err := h.captchaGen.Generate()
	if err != nil {
		Fail(c, http.StatusInternalServerError, 2001, "验证码生成失败")
		return
	}
	_ = answer
	OK(c, gin.H{"enabled": true, "captchaId": id, "image": b64})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, 1004, "请求参数不完整")
		return
	}
	accessToken, refreshToken, user, err := h.auth.Login(
		c.Request.Context(), req.Username, req.Password, req.CaptchaID, req.CaptchaCode, c.ClientIP(),
	)
	switch {
	case errors.Is(err, service.ErrCaptchaRequired):
		Fail(c, http.StatusBadRequest, 1003, "请输入验证码")
	case errors.Is(err, service.ErrCaptchaInvalid):
		Fail(c, http.StatusBadRequest, 1003, "验证码错误或已过期")
	case errors.Is(err, service.ErrLocked):
		Fail(c, http.StatusLocked, 1002, "账号已锁定，请稍后重试或联系管理员解锁")
	case errors.Is(err, service.ErrBadCredentials):
		Fail(c, http.StatusUnauthorized, 1001, "用户名或密码错误")
	case err != nil:
		Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
	default:
		OK(c, gin.H{
			"accessToken":  accessToken,
			"refreshToken": refreshToken,
			"user":         user,
		})
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, 1004, "请求参数不完整")
		return
	}
	user, err := h.auth.Register(c.Request.Context(), req.Username, req.Email, req.Password)
	switch {
	case errors.Is(err, service.ErrWeakPassword):
		settings, _ := h.settingsSvc.Get(c.Request.Context())
		issues := service.ValidatePassword(req.Password, settings)
		FailWithField(c, http.StatusBadRequest, 1004, "密码不符合安全策略", issues)
	case errors.Is(err, service.ErrUserExists):
		FailWithField(c, http.StatusConflict, 1009, "用户名或邮箱已被占用", map[string]string{
			"username": "该用户名或邮箱已注册",
		})
	case err != nil:
		Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
	default:
		OK(c, gin.H{"user": user})
	}
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req forgotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, 1004, "请输入有效的邮箱地址")
		return
	}
	if err := h.auth.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	OK(c, gin.H{"message": "如果该邮箱已注册，重置链接已发送"})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req resetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, 1004, "请求参数不完整")
		return
	}
	err := h.auth.ResetPassword(c.Request.Context(), req.Token, req.NewPassword)
	switch {
	case errors.Is(err, store.ErrNotFound):
		Fail(c, http.StatusBadRequest, 1010, "重置链接无效或已过期，请重新申请")
	case errors.Is(err, service.ErrWeakPassword):
		settings, _ := h.settingsSvc.Get(c.Request.Context())
		issues := service.ValidatePassword(req.NewPassword, settings)
		FailWithField(c, http.StatusBadRequest, 1004, "密码不符合安全策略", issues)
	case err != nil:
		Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
	default:
		OK(c, gin.H{"message": "密码重置成功，请使用新密码登录"})
	}
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, 1007, "缺少刷新令牌")
		return
	}
	accessToken, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		Fail(c, http.StatusUnauthorized, 1007, "登录状态已过期，请重新登录")
		return
	}
	OK(c, gin.H{"accessToken": accessToken})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		OK(c, gin.H{"message": "已登出"})
		return
	}
	_ = h.auth.Logout(c.Request.Context(), req.RefreshToken)
	OK(c, gin.H{"message": "已登出"})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetString("userID")
	user, err := h.auth.FindUserByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		Fail(c, http.StatusUnauthorized, 1005, "登录状态无效")
		return
	}
	OK(c, gin.H{"user": user, "serverTime": time.Now().Format(time.RFC3339)})
}

func (h *AuthHandler) PasswordPolicy(c *gin.Context) {
	settings, err := h.settingsSvc.Get(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	OK(c, gin.H{
		"passwordMinLength":      settings.PasswordMinLength,
		"passwordRequireUpper":   settings.PasswordRequireUpper,
		"passwordRequireLower":   settings.PasswordRequireLower,
		"passwordRequireDigit":   settings.PasswordRequireDigit,
		"passwordRequireSpecial": settings.PasswordRequireSpecial,
		"passwordMinCategories":  settings.PasswordMinCategories,
	})
}

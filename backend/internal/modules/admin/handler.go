package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/model"
	"starry/backend/internal/service"
	"starry/backend/internal/store"
)

type Handler struct {
	settingsSvc *service.SettingsService
	adminSvc    *AdminService
}

func New(settingsSvc *service.SettingsService, adminSvc *AdminService) *Handler {
	return &Handler{settingsSvc: settingsSvc, adminSvc: adminSvc}
}

type updateSettingsRequest struct {
	PasswordMinLength      int  `json:"passwordMinLength"`
	PasswordRequireUpper   bool `json:"passwordRequireUpper"`
	PasswordRequireLower   bool `json:"passwordRequireLower"`
	PasswordRequireDigit   bool `json:"passwordRequireDigit"`
	PasswordRequireSpecial bool `json:"passwordRequireSpecial"`
	PasswordMinCategories  int  `json:"passwordMinCategories"`
	LockoutThreshold       int  `json:"lockoutThreshold"`
	LockoutDurationMinutes int  `json:"lockoutDurationMinutes"`
	AccessTokenMinutes     int  `json:"accessTokenMinutes"`
	RefreshTokenDays       int  `json:"refreshTokenDays"`
	CaptchaEnabled         bool `json:"captchaEnabled"`
}

func (h *Handler) GetSettings(c *gin.Context) {
	settings, err := h.settingsSvc.Get(c.Request.Context())
	if err != nil {
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	core.OK(c, settings)
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, 1004, "请求参数格式错误")
		return
	}
	in := model.AuthSettings{
		PasswordMinLength:      req.PasswordMinLength,
		PasswordRequireUpper:   req.PasswordRequireUpper,
		PasswordRequireLower:   req.PasswordRequireLower,
		PasswordRequireDigit:   req.PasswordRequireDigit,
		PasswordRequireSpecial: req.PasswordRequireSpecial,
		PasswordMinCategories:  req.PasswordMinCategories,
		LockoutThreshold:       req.LockoutThreshold,
		LockoutDurationMinutes: req.LockoutDurationMinutes,
		AccessTokenMinutes:     req.AccessTokenMinutes,
		RefreshTokenDays:       req.RefreshTokenDays,
		CaptchaEnabled:         req.CaptchaEnabled,
	}
	enabledCategories := 0
	for _, b := range []bool{req.PasswordRequireUpper, req.PasswordRequireLower, req.PasswordRequireDigit, req.PasswordRequireSpecial} {
		if b {
			enabledCategories++
		}
	}
	if req.PasswordMinCategories > enabledCategories {
		core.FailWithField(c, http.StatusBadRequest, 1008, "保存失败", map[string]string{
			"passwordMinCategories": "字符类别数不能超过已启用的类别数",
		})
		return
	}
	updated, issues, err := h.settingsSvc.Update(c.Request.Context(), in)
	switch {
	case err != nil:
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
	case len(issues) > 0:
		core.FailWithField(c, http.StatusBadRequest, 1008, "参数超出允许范围", issues)
	default:
		core.OK(c, updated)
	}
}

func (h *Handler) ListUsers(c *gin.Context) {
	search := c.Query("search")
	users, err := h.adminSvc.ListUsers(c.Request.Context(), search)
	if err != nil {
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	core.OK(c, gin.H{"users": users})
}

func (h *Handler) GetUser(c *gin.Context) {
	userID := c.Param("id")
	detail, err := h.adminSvc.GetUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	core.OK(c, detail)
}

func (h *Handler) UnlockUser(c *gin.Context) {
	userID := c.Param("id")
	if err := h.adminSvc.UnlockUser(c.Request.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	core.OK(c, gin.H{"message": "已解除锁定并清零失败计数"})
}

func (h *Handler) ForceResetPassword(c *gin.Context) {
	userID := c.Param("id")
	token, err := h.adminSvc.ForceResetPassword(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	core.OK(c, gin.H{"token": token, "message": "重置令牌已生成（30 分钟内有效，单次使用）"})
}

func (h *Handler) RevokeSessions(c *gin.Context) {
	userID := c.Param("id")
	if err := h.adminSvc.RevokeSessions(c.Request.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	core.OK(c, gin.H{"message": "已吊销该用户全部在线会话"})
}

func (h *Handler) FreezeUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == c.GetString("userID") {
		core.Fail(c, http.StatusBadRequest, 1009, "不能冻结当前登录的管理员账号")
		return
	}
	if err := h.adminSvc.FreezeUser(c.Request.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		if errors.Is(err, service.ErrAdminUser) {
			core.Fail(c, http.StatusBadRequest, 1009, "管理员账号不允许冻结")
			return
		}
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	core.OK(c, gin.H{"message": "账号已冻结，该用户全部会话已吊销"})
}

func (h *Handler) UnfreezeUser(c *gin.Context) {
	userID := c.Param("id")
	if err := h.adminSvc.UnfreezeUser(c.Request.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	core.OK(c, gin.H{"message": "账号已解冻，可正常登录"})
}

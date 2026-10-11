package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/model"
	"starry/backend/internal/request"
	"starry/backend/internal/service"
	"starry/backend/internal/store"
)

type Handler struct {
	settingsSvc *service.SettingsService
	adminSvc    *AdminService
	db          *store.DB
}

func New(settingsSvc *service.SettingsService, adminSvc *AdminService, db *store.DB) *Handler {
	return &Handler{settingsSvc: settingsSvc, adminSvc: adminSvc, db: db}
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

// @Summary      读取登录安全策略
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{}
// @Failure      503 {object} map[string]interface{}
// @Router       /admin/settings [get]
func (h *Handler) GetSettings(c *gin.Context) {
	settings, err := h.settingsSvc.Get(c.Request.Context())
	if err != nil {
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	core.OK(c, settings)
}

// @Summary      更新登录安全策略（即时生效）
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} map[string]interface{}
// @Router       /admin/settings [put]
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
		h.recordAudit(c, "settings.update", "", "failed", err.Error())
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
	case len(issues) > 0:
		core.FailWithField(c, http.StatusBadRequest, 1008, "参数超出允许范围", issues)
	default:
		h.recordAudit(c, "settings.update", "", "success", "")
		core.OK(c, updated)
	}
}

// @Summary      用户列表（搜索 + 分页）
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        search  query  string  false  "用户名或邮箱关键词"
// @Param        page    query  integer false  "页码，默认 1"
// @Param        size    query  integer false  "每页条数，默认 50，上限 200"
// @Success      200     {object} map[string]interface{}
// @Failure      503     {object} map[string]interface{}
// @Router       /admin/users [get]
func (h *Handler) ListUsers(c *gin.Context) {
	search := c.Query("search")
	size := request.IntParam(c, "size", 50, 1, 200)
	page := request.IntParam(c, "page", 1, 1, 100000)
	offset := (page - 1) * size
	users, err := h.adminSvc.ListUsers(c.Request.Context(), search, size, offset)
	if err != nil {
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	total, err := h.db.CountUsers(search)
	if err != nil {
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	core.OK(c, gin.H{"users": users, "total": total})
}

// @Summary      用户详情
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "用户 ID"
// @Success      200 {object} map[string]interface{}
// @Failure      404 {object} map[string]interface{}
// @Router       /admin/users/{id} [get]
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

// @Summary      解除用户锁定并清零失败计数
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "用户 ID"
// @Success      200 {object} map[string]interface{}
// @Failure      404 {object} map[string]interface{}
// @Router       /admin/users/{id}/unlock [post]
func (h *Handler) UnlockUser(c *gin.Context) {
	userID := c.Param("id")
	if err := h.adminSvc.UnlockUser(c.Request.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.recordAudit(c, "user.unlock", userID, "failed", "user not found")
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		h.recordAudit(c, "user.unlock", userID, "failed", err.Error())
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	h.recordAudit(c, "user.unlock", userID, "success", "")
	core.OK(c, gin.H{"message": "已解除锁定并清零失败计数"})
}

// @Summary      强制重置用户密码（返回一次性令牌）
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "用户 ID"
// @Success      200 {object} map[string]interface{}
// @Failure      404 {object} map[string]interface{}
// @Router       /admin/users/{id}/reset-password [post]
func (h *Handler) ForceResetPassword(c *gin.Context) {
	userID := c.Param("id")
	token, err := h.adminSvc.ForceResetPassword(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.recordAudit(c, "user.reset_password", userID, "failed", "user not found")
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		h.recordAudit(c, "user.reset_password", userID, "failed", err.Error())
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	h.recordAudit(c, "user.reset_password", userID, "success", "reset token issued")
	core.OK(c, gin.H{"token": token, "message": "重置令牌已生成（30 分钟内有效，单次使用）"})
}

// @Summary      吊销用户全部会话
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "用户 ID"
// @Success      200 {object} map[string]interface{}
// @Failure      404 {object} map[string]interface{}
// @Router       /admin/users/{id}/revoke-sessions [post]
func (h *Handler) RevokeSessions(c *gin.Context) {
	userID := c.Param("id")
	if err := h.adminSvc.RevokeSessions(c.Request.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.recordAudit(c, "user.revoke_sessions", userID, "failed", "user not found")
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		h.recordAudit(c, "user.revoke_sessions", userID, "failed", err.Error())
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	h.recordAudit(c, "user.revoke_sessions", userID, "success", "")
	core.OK(c, gin.H{"message": "已吊销该用户全部在线会话"})
}

// @Summary      冻结账号并吊销会话
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "用户 ID"
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} map[string]interface{}
// @Router       /admin/users/{id}/freeze [post]
func (h *Handler) FreezeUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == c.GetString("userID") {
		h.recordAudit(c, "user.freeze", userID, "denied", "attempt to freeze self")
		core.Fail(c, http.StatusBadRequest, 1009, "不能冻结当前登录的管理员账号")
		return
	}
	if err := h.adminSvc.FreezeUser(c.Request.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.recordAudit(c, "user.freeze", userID, "failed", "user not found")
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		if errors.Is(err, service.ErrAdminUser) {
			h.recordAudit(c, "user.freeze", userID, "denied", "admin account not freezable")
			core.Fail(c, http.StatusBadRequest, 1009, "管理员账号不允许冻结")
			return
		}
		h.recordAudit(c, "user.freeze", userID, "failed", err.Error())
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	h.recordAudit(c, "user.freeze", userID, "success", "")
	core.OK(c, gin.H{"message": "账号已冻结，该用户全部会话已吊销"})
}

// @Summary      解冻账号
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "用户 ID"
// @Success      200 {object} map[string]interface{}
// @Failure      404 {object} map[string]interface{}
// @Router       /admin/users/{id}/unfreeze [post]
func (h *Handler) UnfreezeUser(c *gin.Context) {
	userID := c.Param("id")
	if err := h.adminSvc.UnfreezeUser(c.Request.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.recordAudit(c, "user.unfreeze", userID, "failed", "user not found")
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		h.recordAudit(c, "user.unfreeze", userID, "failed", err.Error())
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	h.recordAudit(c, "user.unfreeze", userID, "success", "")
	core.OK(c, gin.H{"message": "账号已解冻，可正常登录"})
}

// DeleteUser 安全删除用户：吊销其全部会话，级联清理其所有业务数据（含网盘与
// 笔记附件的磁盘文件），最后硬删账号。不允许删除自己，也不允许删除管理员。
// @Summary      删除用户（级联清理其业务数据与磁盘文件）
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "用户 ID"
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} map[string]interface{}
// @Router       /admin/users/{id} [delete]
func (h *Handler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == c.GetString("userID") {
		h.recordAudit(c, "user.delete", userID, "denied", "attempt to delete self")
		core.Fail(c, http.StatusBadRequest, 1009, "不能删除当前登录的管理员账号")
		return
	}
	if err := h.adminSvc.DeleteUser(c.Request.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.recordAudit(c, "user.delete", userID, "failed", "user not found")
			core.Fail(c, http.StatusNotFound, 1011, "用户不存在")
			return
		}
		if errors.Is(err, service.ErrAdminUser) {
			h.recordAudit(c, "user.delete", userID, "denied", "admin account not deletable")
			core.Fail(c, http.StatusBadRequest, 1009, "管理员账号不允许删除")
			return
		}
		h.recordAudit(c, "user.delete", userID, "failed", err.Error())
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	h.recordAudit(c, "user.delete", userID, "success", "")
	core.OK(c, gin.H{"message": "用户及其全部数据已删除"})
}

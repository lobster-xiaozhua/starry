package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
	"starry/backend/internal/logx"
	"starry/backend/internal/middleware"
	"starry/backend/internal/model"
	"starry/backend/internal/request"
)

// recordAudit 把一次管理操作写入审计日志（best-effort）。
//
// 调用方在业务操作成功或失败后调用本方法；写库失败只告警，绝不阻断已被审计的主业务 ——
// 否则审计子系统抖动会反噬可用性，等于用安全特性制造了新的 DoS 面。
// 目标名称（targetName）做写时快照：即便用户之后被删或改名，审计条目仍然可读。
func (h *Handler) recordAudit(c *gin.Context, action, targetID, outcome, detail string) {
	targetName := ""
	if targetID != "" {
		if u, err := h.db.FindUserByID(targetID); err == nil && u != nil {
			targetName = u.Username
		}
	}
	entry := &model.AuditLog{
		Action:     action,
		ActorID:    c.GetString("userID"),
		ActorName:  c.GetString("username"),
		TargetID:   targetID,
		TargetName: targetName,
		Outcome:    outcome,
		Detail:     detail,
		IP:         c.ClientIP(),
		RequestID:  middleware.RequestIDFrom(c),
	}
	if err := h.db.AppendAudit(c.Request.Context(), entry); err != nil {
		logx.L().Warn("audit log write failed", "action", action, "target", targetID, "error", err.Error())
	}
}

// ListAuditLogs GET /api/admin/audit
// 拉取管理操作审计记录，支持 ?actor=&action= 过滤与分页（分页参数经 request 包钳制）。
func (h *Handler) ListAuditLogs(c *gin.Context) {
	page := request.Page(c)
	size := request.Size(c, 50, 200)
	logs, total, err := h.db.ListAudit(c.Request.Context(), c.Query("actor"), c.Query("action"), size, (page-1)*size)
	if err != nil {
		core.Fail(c, http.StatusServiceUnavailable, 2001, "系统繁忙，请稍后重试")
		return
	}
	core.OK(c, gin.H{"logs": logs, "total": total, "page": page, "size": size})
}

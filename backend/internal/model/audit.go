package model

import "time"

// AuditLog 记录一次「有安全影响」的操作，目前由管理端写入：
// 冻结/解冻账号、解除锁定、强制改密、吊销会话、修改系统安全设置等。
//
// 设计取舍：
//   - 操作人（actor）与目标（target）都存 ID + 名称快照，使审计记录在用户被删除或改名后
//     依然可读，不必再做 join；名称是写时快照，天然防篡改。
//   - outcome 区分 success / failed / denied，把「尝试但被拒」（如管理员试图冻结自己）
//     也留痕 —— 这类越权尝试往往比成功的操作更有调查价值。
//   - 审计写入是 best-effort：调用方在业务成功/失败后记录，写库失败只告警不阻断主流程，
//     否则审计子系统抖动会反噬主业务可用性。
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	Action     string    `gorm:"size:48;not null;index" json:"action"` // 如 user.freeze / settings.update
	ActorID    string    `gorm:"size:48;index" json:"actorId"`         // 操作人 userID
	ActorName  string    `gorm:"size:64" json:"actorName"`             // 操作人名称快照
	TargetID   string    `gorm:"size:48;index" json:"targetId"`        // 被操作对象 userID（系统级操作可为空）
	TargetName string    `gorm:"size:64" json:"targetName"`            // 被操作对象名称快照
	Outcome    string    `gorm:"size:16;not null" json:"outcome"`      // success | failed | denied
	Detail     string    `gorm:"size:1024" json:"detail"`              // 自由文本，记录关键上下文
	IP         string    `gorm:"size:64" json:"ip"`                    // 操作来源 IP
	RequestID  string    `gorm:"size:64" json:"requestId"`             // 关联访问日志的追踪码
}

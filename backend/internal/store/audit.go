package store

import (
	"context"

	"starry/backend/internal/model"
)

// MigrateAudit 建审计表。审计由 admin 模块拥有（它是系统安全操作的天然归属），
// 因此挂在 admin.Migrate 下，与模块开关保持一致：admin 停用时既不挂路由也不迁表。
func (s *DB) MigrateAudit() error {
	return s.gorm.AutoMigrate(&model.AuditLog{})
}

// AppendAudit 追加一条审计记录。best-effort：调用方不依赖其成功，
// 写入失败由调用方记录告警，绝不阻断被审计的主操作。
func (s *DB) AppendAudit(ctx context.Context, log *model.AuditLog) error {
	return s.gorm.WithContext(ctx).Create(log).Error
}

// ListAudit 分页拉取审计记录，默认按时间倒序（最近的操作在最前）。
// actor / action 为空表示不过滤；limit <= 0 时回落到 50 条上限由调用方约束。
func (s *DB) ListAudit(ctx context.Context, actor, action string, limit, offset int) ([]model.AuditLog, int64, error) {
	q := s.gorm.WithContext(ctx).Model(&model.AuditLog{})
	if actor != "" {
		q = q.Where("actor_id = ?", actor)
	}
	if action != "" {
		q = q.Where("action = ?", action)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	var logs []model.AuditLog
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

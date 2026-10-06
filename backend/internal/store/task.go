package store

import (
	"time"

	"github.com/google/uuid"

	"login-system/backend/internal/model"
)

// CreateTask 插入一条新的长程任务（默认状态 queued）。
func (s *DB) CreateTask(task *model.AgentTask) error {
	return s.gorm.Create(task).Error
}

// GetTask 按用户与 ID 取任务（防止越权访问他人任务）。
func (s *DB) GetTask(userID, taskID uuid.UUID) (*model.AgentTask, error) {
	var t model.AgentTask
	err := s.gorm.Where("user_id = ? AND id = ?", userID, taskID).First(&t).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTasks 返回用户最近的任务（按时间倒序）。
func (s *DB) ListTasks(userID uuid.UUID, limit int) ([]model.AgentTask, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var tasks []model.AgentTask
	err := s.gorm.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&tasks).Error
	return tasks, err
}

// UpdateTaskFields 局部更新任务字段，并刷新 updated_at。
func (s *DB) UpdateTaskFields(userID, taskID uuid.UUID, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now()
	return s.gorm.Model(&model.AgentTask{}).
		Where("user_id = ? AND id = ?", userID, taskID).
		Updates(fields).Error
}

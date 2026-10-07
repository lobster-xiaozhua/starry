package service

import (
	"context"

	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

type SettingsService struct {
	db  *store.DB
	rds *store.Redis
}

func NewSettingsService(db *store.DB, rds *store.Redis) *SettingsService {
	return &SettingsService{db: db, rds: rds}
}

func (s *SettingsService) Get(ctx context.Context) (model.AuthSettings, error) {
	return s.db.LoadSettings()
}

func (s *SettingsService) Update(ctx context.Context, in model.AuthSettings) (model.AuthSettings, ValidationIssues, error) {
	if issues := ValidateSettingsUpdate(in); len(issues) > 0 {
		return in, issues, nil
	}
	if err := s.db.SaveSettings(&in); err != nil {
		return in, nil, err
	}
	if b, err := marshalJSON(in); err == nil {
		_ = s.rds.SetCache(ctx, "settings:auth", b)
	}
	return in, nil, nil
}

func (s *SettingsService) InvalidateCache(ctx context.Context) {
	_ = s.rds.Delete(ctx, "settings:auth")
}

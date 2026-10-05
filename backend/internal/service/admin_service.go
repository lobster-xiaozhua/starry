package service

import (
	"context"
	"time"

	"login-system/backend/internal/authpkg"
	"login-system/backend/internal/model"
	"login-system/backend/internal/store"
)

type AdminService struct {
	db     *store.DB
	rds    *store.Redis
	mailer func(email, token string)
}

func NewAdminService(db *store.DB, rds *store.Redis) *AdminService {
	return &AdminService{db: db, rds: rds, mailer: LogResetToken}
}

func (s *AdminService) SetMailer(m func(email, token string)) {
	s.mailer = m
}

type AdminUserView struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	Locked     bool   `json:"locked"`
	FailCount  int64   `json:"failCount"`
	CreatedAt  string `json:"createdAt"`
}

type AdminUserDetail struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	Locked    bool   `json:"locked"`
	FailCount int64  `json:"failCount"`
	LockTTL   int64  `json:"lockTtl"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func (s *AdminService) ListUsers(ctx context.Context, search string) ([]AdminUserView, error) {
	users, err := s.db.ListUsers(search)
	if err != nil {
		return nil, err
	}
	views := make([]AdminUserView, 0, len(users))
	for _, u := range users {
		locked, _ := s.rds.IsLocked(ctx, u.ID.String())
		fail, _ := s.rds.GetFailureCount(ctx, u.ID.String())
		views = append(views, AdminUserView{
			ID:        u.ID.String(),
			Username:  u.Username,
			Email:     u.Email,
			Role:      u.Role,
			Status:    u.Status,
			Locked:    locked,
			FailCount: fail,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		})
	}
	return views, nil
}

func (s *AdminService) GetUser(ctx context.Context, userID string) (*AdminUserDetail, error) {
	user, err := s.db.FindUserByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, store.ErrNotFound
	}
	locked, _ := s.rds.IsLocked(ctx, user.ID.String())
	fail, _ := s.rds.GetFailureCount(ctx, user.ID.String())
	lockTTL := int64(0)
	if locked {
		if ttl, err := s.rds.LockTTL(ctx, user.ID.String()); err == nil && ttl > 0 {
			lockTTL = int64(ttl.Seconds())
		}
	}
	return &AdminUserDetail{
		ID:        user.ID.String(),
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		Status:    user.Status,
		Locked:    locked,
		FailCount: fail,
		LockTTL:   lockTTL,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
		UpdatedAt: user.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *AdminService) UnlockUser(ctx context.Context, userID string) error {
	user, err := s.db.FindUserByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return store.ErrNotFound
	}
	return s.rds.UnlockUser(ctx, userID)
}

func (s *AdminService) ForceResetPassword(ctx context.Context, userID string) (string, error) {
	user, err := s.db.FindUserByID(userID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", store.ErrNotFound
	}
	token, err := authpkg.GenerateOpaqueToken()
	if err != nil {
		return "", err
	}
	if err := s.rds.SetResetToken(ctx, token, user.ID.String(), 30*time.Minute); err != nil {
		return "", err
	}
	s.mailer(user.Email, token)
	return token, nil
}

func (s *AdminService) RevokeSessions(ctx context.Context, userID string) error {
	if err := s.rds.RevokeAllUserTokens(ctx, userID, 500); err != nil {
		return err
	}
	if err := s.rds.UnlockUser(ctx, userID); err != nil {
		return err
	}
	return nil
}

var _ = model.User{}

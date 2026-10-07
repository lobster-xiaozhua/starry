package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"starry/backend/internal/authpkg"
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

type AuthService struct {
	db            *store.DB
	rds           *store.Redis
	jwtSecret     string
	mailer        func(email, token string)
	verifyCaptcha func(ctx context.Context, captchaID, answer string) bool
}

func NewAuthService(db *store.DB, rds *store.Redis, jwtSecret string) *AuthService {
	return &AuthService{db: db, rds: rds, jwtSecret: jwtSecret, mailer: LogResetToken}
}

func (s *AuthService) Settings(ctx context.Context) (model.AuthSettings, error) {
	if b, err := s.rds.GetCache(ctx, "settings:auth"); err == nil && b != nil {
		var settings model.AuthSettings
		if json.Unmarshal(b, &settings) == nil {
			return settings, nil
		}
	}
	settings, err := s.db.LoadSettings()
	if err != nil {
		return settings, err
	}
	s.refreshSettingsCache(ctx, settings)
	return settings, nil
}

func (s *AuthService) refreshSettingsCache(ctx context.Context, settings model.AuthSettings) {
	if b, err := json.Marshal(settings); err == nil {
		_ = s.rds.SetCache(ctx, "settings:auth", b)
	}
}

func (s *AuthService) Login(ctx context.Context, username, password, captchaID, captchaCode, clientIP string) (string, string, *model.User, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return "", "", nil, err
	}
	if settings.CaptchaEnabled {
		if captchaID == "" || captchaCode == "" {
			return "", "", nil, ErrCaptchaRequired
		}
		if !s.verifyCaptcha(ctx, captchaID, captchaCode) {
			return "", "", nil, ErrCaptchaInvalid
		}
	}
	user, err := s.db.FindUserByUsername(username)
	if err != nil {
		return "", "", nil, err
	}
	locked, err := s.rds.IsLocked(ctx, userIdentifier(user))
	if err != nil {
		return "", "", nil, err
	}
	if locked {
		return "", "", nil, ErrLocked
	}
	if user == nil || !authpkg.CheckPassword(user.PasswordHash, password) {
		if user != nil {
			if err := s.recordFailure(ctx, user.ID.String(), settings); err != nil {
				return "", "", nil, err
			}
		}
		return "", "", nil, ErrBadCredentials
	}
	if user.Status != "active" {
		return "", "", nil, ErrBadCredentials
	}
	if err := s.rds.ClearFailure(ctx, user.ID.String()); err != nil {
		return "", "", nil, err
	}
	accessToken, _, err := authpkg.IssueAccessToken(s.jwtSecret, user.ID.String(), user.Role, time.Duration(settings.AccessTokenMinutes)*time.Minute)
	if err != nil {
		return "", "", nil, err
	}
	refreshToken, refreshJTI, err := authpkg.IssueRefreshToken(s.jwtSecret, user.ID.String(), time.Duration(settings.RefreshTokenDays)*24*time.Hour)
	if err != nil {
		return "", "", nil, err
	}
	if err := s.rds.SetRefreshToken(ctx, refreshJTI, user.ID.String(), time.Duration(settings.RefreshTokenDays)*24*time.Hour); err != nil {
		return "", "", nil, err
	}
	fmt.Printf("[AUDIT] login success user=%s ip=%s\n", user.Username, clientIP)
	return accessToken, refreshToken, user, nil
}

func (s *AuthService) recordFailure(ctx context.Context, userID string, settings model.AuthSettings) error {
	count, err := s.rds.IncrementFailure(ctx, userID, time.Duration(settings.LockoutDurationMinutes)*time.Minute)
	if err != nil {
		return err
	}
	if count >= int64(settings.LockoutThreshold) {
		if err := s.rds.SetLocked(ctx, userID, time.Duration(settings.LockoutDurationMinutes)*time.Minute); err != nil {
			return err
		}
		fmt.Printf("[AUDIT] account locked user_id=%s threshold=%d\n", userID, settings.LockoutThreshold)
	}
	return nil
}

func userIdentifier(u *model.User) string {
	if u == nil {
		return "anonymous"
	}
	return u.ID.String()
}

func (s *AuthService) Register(ctx context.Context, username, email, password string) (*model.User, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	if issues := ValidatePassword(password, settings); len(issues) > 0 {
		return nil, ErrWeakPassword
	}
	existing, err := s.db.FindUserByUsername(username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUserExists
	}
	existing, err = s.db.FindUserByEmail(email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUserExists
	}
	hash, err := authpkg.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Role:         "user",
		Status:       "active",
	}
	if err := s.db.CreateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.db.FindUserByEmail(email)
	if err != nil {
		return err
	}
	if user == nil {
		return nil
	}
	token, err := authpkg.GenerateOpaqueToken()
	if err != nil {
		return err
	}
	if err := s.rds.SetResetToken(ctx, token, user.ID.String(), 30*time.Minute); err != nil {
		return err
	}
	s.mailer(user.Email, token)
	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	settings, err := s.Settings(ctx)
	if err != nil {
		return err
	}
	if issues := ValidatePassword(newPassword, settings); len(issues) > 0 {
		return ErrWeakPassword
	}
	userID, err := s.rds.ConsumeResetToken(ctx, token)
	if err != nil || userID == "" {
		return store.ErrNotFound
	}
	hash, err := authpkg.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.db.UpdateUserPassword(userID, hash); err != nil {
		return err
	}
	if err := s.rds.RevokeAllUserTokens(ctx, userID, 500); err != nil {
		return err
	}
	if err := s.rds.UnlockUser(ctx, userID); err != nil {
		return err
	}
	return nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (string, error) {
	claims, err := authpkg.ParseRefreshToken(s.jwtSecret, refreshToken)
	if err != nil {
		return "", errors.Join(ErrBadCredentials, err)
	}
	registeredUser, err := s.rds.GetRefreshToken(ctx, claims.ID)
	if err != nil {
		return "", err
	}
	if registeredUser == "" || registeredUser != claims.UserID {
		return "", ErrBadCredentials
	}
	user, err := s.db.FindUserByID(claims.UserID)
	if err != nil {
		return "", err
	}
	if user == nil || user.Status != "active" {
		return "", ErrBadCredentials
	}
	settings, err := s.Settings(ctx)
	if err != nil {
		return "", err
	}
	accessToken, _, err := authpkg.IssueAccessToken(s.jwtSecret, user.ID.String(), user.Role, time.Duration(settings.AccessTokenMinutes)*time.Minute)
	if err != nil {
		return "", err
	}
	return accessToken, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	claims, err := authpkg.ParseRefreshToken(s.jwtSecret, refreshToken)
	if err != nil {
		return nil
	}
	return s.rds.RevokeRefreshToken(ctx, claims.ID)
}

func (s *AuthService) SetCaptchaVerifier(v func(ctx context.Context, captchaID, answer string) bool) {
	s.verifyCaptcha = v
}

func (s *AuthService) SetMailer(m func(email, token string)) {
	s.mailer = m
}

func (s *AuthService) FindUserByID(ctx context.Context, id string) (*model.User, error) {
	return s.db.FindUserByID(id)
}

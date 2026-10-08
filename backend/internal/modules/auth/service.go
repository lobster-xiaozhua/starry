package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"starry/backend/internal/authpkg"
	"starry/backend/internal/logx"
	"starry/backend/internal/model"
	"starry/backend/internal/service"
	"starry/backend/internal/store"
)

type AuthService struct {
	db            *store.DB
	rds           *store.Redis
	jwtSecret     string
	mailer        func(email, token string)
	verifyCaptcha func(ctx context.Context, captchaID, answer string) bool
}

func NewService(db *store.DB, rds *store.Redis, jwtSecret string) *AuthService {
	return &AuthService{db: db, rds: rds, jwtSecret: jwtSecret, mailer: service.LogResetToken}
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
			return "", "", nil, service.ErrCaptchaRequired
		}
		if !s.verifyCaptcha(ctx, captchaID, captchaCode) {
			return "", "", nil, service.ErrCaptchaInvalid
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
		// 已被锁定仍持续尝试：潜在暴破信号，落审计（durable）便于事后追溯。
		s.securityEvent(ctx, slog.LevelWarn, "auth.login.failed", "denied", true, user, clientIP, "account locked")
		return "", "", nil, service.ErrLocked
	}
	if user == nil || !authpkg.CheckPassword(user.PasswordHash, password) {
		if user != nil {
			count, lockedNow, err := s.recordFailure(ctx, user.ID.String(), settings)
			if err != nil {
				return "", "", nil, err
			}
			// 单次密码错误只走结构化日志（高基数、低信号），避免审计表被暴破刷爆；
			// 是否触发锁定在 recordFailure 内已决定，这里只补一条可观测日志。
			s.securityEvent(ctx, slog.LevelWarn, "auth.login.failed", "failed", false, user, clientIP, fmt.Sprintf("attempts=%d", count))
			if lockedNow {
				s.securityEvent(ctx, slog.LevelWarn, "auth.account.locked", "denied", true, user, clientIP, fmt.Sprintf("threshold=%d", settings.LockoutThreshold))
			}
		}
		return "", "", nil, service.ErrBadCredentials
	}
	if user.Status != "active" {
		// 密码正确但账号非活跃：合法用户被停用，记一条 denied 日志（不落审计，避免噪音）。
		s.securityEvent(ctx, slog.LevelWarn, "auth.login.denied", "denied", false, user, clientIP, "account inactive")
		return "", "", nil, service.ErrBadCredentials
	}
	if err := s.rds.ClearFailure(ctx, user.ID.String()); err != nil {
		return "", "", nil, err
	}
	accessToken, _, err := authpkg.IssueAccessToken(s.jwtSecret, user.ID.String(), user.Username, user.Role, time.Duration(settings.AccessTokenMinutes)*time.Minute)
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
	s.securityEvent(ctx, slog.LevelInfo, "auth.login.success", "success", true, user, clientIP, "")
	return accessToken, refreshToken, user, nil
}

// recordFailure 累加失败计数，并在达到阈值时锁定账号。返回当前失败次数与本次是否触发锁定，
// 供 Login 决定补记哪类安全事件。
func (s *AuthService) recordFailure(ctx context.Context, userID string, settings model.AuthSettings) (count int64, lockedNow bool, err error) {
	count, err = s.rds.IncrementFailure(ctx, userID, time.Duration(settings.LockoutDurationMinutes)*time.Minute)
	if err != nil {
		return 0, false, err
	}
	if count >= int64(settings.LockoutThreshold) {
		if err := s.rds.SetLocked(ctx, userID, time.Duration(settings.LockoutDurationMinutes)*time.Minute); err != nil {
			return count, false, err
		}
		return count, true, nil
	}
	return count, false, nil
}

// securityEvent 记录一条认证安全事件。
//
// level 决定日志级别（成功 Info、各类失败/锁定 Warn）。durable=true 时额外写入审计表，
// 落库为 best-effort：写库失败只告警，绝不阻断登录主流程（否则审计子系统抖动会反噬可用性）。
// 设计取舍：高基数、低信号的事件（如单次密码错误）只走结构化日志，便于日志管线做时间序列
// 聚合；只有登录成功、账号被锁、被锁后持续尝试这类有调查价值的事件才落审计表持久化。
func (s *AuthService) securityEvent(ctx context.Context, level slog.Level, action, outcome string, durable bool, target *model.User, ip, detail string) {
	entry := buildAuthAudit(action, outcome, target, ip, detail)

	attrs := []slog.Attr{
		slog.String("event", action),
		slog.String("outcome", outcome),
		slog.String("user_id", entry.ActorID),
		slog.String("username", entry.ActorName),
		slog.String("ip", ip),
	}
	if detail != "" {
		attrs = append(attrs, slog.String("detail", detail))
	}
	logx.L().LogAttrs(ctx, level, "auth security event", attrs...)

	if !durable {
		return
	}
	if err := s.db.AppendAudit(ctx, entry); err != nil {
		logx.L().Warn("audit write failed", slog.String("action", action), slog.String("error", err.Error()))
	}
}

// buildAuthAudit 纯函数：根据认证事件构造审计条目。与行为解耦便于单测覆盖
// 已知用户 / 未知用户（防枚举）等不同分支的字段取值。
func buildAuthAudit(action, outcome string, target *model.User, ip, detail string) *model.AuditLog {
	userID, username := "", ""
	if target != nil {
		userID = target.ID.String()
		username = target.Username
	}
	return &model.AuditLog{
		Action:     action,
		ActorID:    userID,
		ActorName:  username,
		TargetID:   userID,
		TargetName: username,
		Outcome:    outcome,
		Detail:     detail,
		IP:         ip,
	}
}

// LockoutView 是只读锁定视图中的单项，供管理员观察当前被锁账号与剩余锁定时长。
type LockoutView struct {
	UserID         string `json:"userId"`
	Username       string `json:"username"`
	Attempts       int64  `json:"attempts"`
	LockTTLSeconds int64  `json:"lockTtlSeconds"`
}

// ListLockouts 返回当前真正处于锁定状态的账号（自然过期的条目会被过滤掉），
// 含失败次数与剩余锁定秒数。只读、用于安全可观测，不改变任何状态。
func (s *AuthService) ListLockouts(ctx context.Context) ([]LockoutView, error) {
	members, err := s.rds.ListLockedMembers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LockoutView, 0, len(members))
	for _, id := range members {
		locked, err := s.rds.IsLocked(ctx, id)
		if err != nil || !locked {
			continue // 跳过已自然过期的成员，保证视图与实时锁定态一致
		}
		v := LockoutView{UserID: id}
		if u, e := s.db.FindUserByID(id); e == nil && u != nil {
			v.Username = u.Username
		}
		if c, e := s.rds.GetFailureCount(ctx, id); e == nil {
			v.Attempts = c
		}
		if ttl, e := s.rds.LockTTL(ctx, id); e == nil && ttl > 0 {
			v.LockTTLSeconds = int64(ttl.Seconds())
		}
		out = append(out, v)
	}
	return out, nil
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
	if issues := service.ValidatePassword(password, settings); len(issues) > 0 {
		return nil, service.ErrWeakPassword
	}
	existing, err := s.db.FindUserByUsername(username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, service.ErrUserExists
	}
	existing, err = s.db.FindUserByEmail(email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, service.ErrUserExists
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
	if issues := service.ValidatePassword(newPassword, settings); len(issues) > 0 {
		return service.ErrWeakPassword
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
		return "", errors.Join(service.ErrBadCredentials, err)
	}
	registeredUser, err := s.rds.GetRefreshToken(ctx, claims.ID)
	if err != nil {
		return "", err
	}
	if registeredUser == "" || registeredUser != claims.UserID {
		return "", service.ErrBadCredentials
	}
	user, err := s.db.FindUserByID(claims.UserID)
	if err != nil {
		return "", err
	}
	if user == nil || user.Status != "active" {
		return "", service.ErrBadCredentials
	}
	settings, err := s.Settings(ctx)
	if err != nil {
		return "", err
	}
	accessToken, _, err := authpkg.IssueAccessToken(s.jwtSecret, user.ID.String(), user.Username, user.Role, time.Duration(settings.AccessTokenMinutes)*time.Minute)
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

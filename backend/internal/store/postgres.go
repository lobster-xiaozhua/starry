package store

import (
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"starry/backend/internal/model"
)

type DB struct {
	gorm *gorm.DB
}

func NewPostgres(dsn string) (*DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &DB{gorm: db}, nil
}

// Ping 探活 PostgreSQL，供 /health 健康检查使用。
func (s *DB) Ping() error {
	var one int
	return s.gorm.Raw("SELECT 1").Scan(&one).Error
}

// EnsureExtensions 创建迁移所需的扩展，必须在任何建表动作之前执行：
// knowledge_chunks 的 vector(384) 列依赖 pgvector，笔记标题模糊搜索依赖 pg_trgm。
func (s *DB) EnsureExtensions() error {
	for _, stmt := range []string{
		"CREATE EXTENSION IF NOT EXISTS pg_trgm",
		"CREATE EXTENSION IF NOT EXISTS pgcrypto",
		"CREATE EXTENSION IF NOT EXISTS vector",
	} {
		if err := s.gorm.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

// AutoMigrate 对给定模型建表/补列。
//
// 各业务模块在自己的 Migrate 中声明「本模块拥有的模型」，由 modules.MigrateAll
// 按序统一编排。这样 schema 的归属与模块一致：改动某个模块的表不会再牵动他人，
// 且停用的模块可选择不迁移其表。注意此处只做编排拆分，不重命名任何既有表，
// 因此不会破坏已部署环境的数据。
func (s *DB) AutoMigrate(models ...any) error {
	if len(models) == 0 {
		return nil
	}
	return s.gorm.AutoMigrate(models...)
}

// ExecDDL 执行模块自带的 DDL（生成列、索引等），供各模块 Migrate 使用。
func (s *DB) ExecDDL(stmts ...string) error {
	for _, stmt := range stmts {
		if err := s.gorm.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

// ExecDDLBestEffort 执行「有则更好、无则降级」的 DDL，失败只告警不阻断启动。
//
// 适用对象：依赖特定扩展版本的可选索引（如 pgvector 的 hnsw）。托管数据库版本参差，
// 把「索引建不出来」升级成「服务起不来」是典型的可用性事故。
func (s *DB) ExecDDLBestEffort(stmts ...string) {
	for _, stmt := range stmts {
		if err := s.gorm.Exec(stmt).Error; err != nil {
			slog.Warn("optional DDL skipped", "stmt", stmt, "error", err.Error())
		}
	}
}

func (s *DB) SeedAdmin(username, email, passwordHash string) error {
	var count int64
	if err := s.gorm.Model(&model.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	admin := model.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         "admin",
		Status:       "active",
	}
	return s.gorm.Create(&admin).Error
}

// SeedSettings 首次启动时写入默认安全策略。captchaEnabled 来自配置（CAPTCHA_ENABLED），
// 仅在建库这一次生效——之后由管理端设置接管，避免重启把管理员的选择覆盖回去。
// 自动化环境（CI / 冒烟）可把它关掉以获得确定的登录路径。
func (s *DB) SeedSettings(captchaEnabled bool) error {
	settings := model.DefaultAuthSettings()
	settings.CaptchaEnabled = captchaEnabled
	return s.gorm.Where("id = ?", 1).Attrs(settings).FirstOrCreate(&settings).Error
}

func (s *DB) FindUserByUsername(username string) (*model.User, error) {
	var u model.User
	err := s.gorm.Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *DB) FindUserByEmail(email string) (*model.User, error) {
	var u model.User
	err := s.gorm.Where("email = ?", email).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *DB) FindUserByID(id string) (*model.User, error) {
	var u model.User
	err := s.gorm.Where("id = ?", id).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *DB) CreateUser(u *model.User) error {
	return s.gorm.Create(u).Error
}

func (s *DB) UpdateUserPassword(id, passwordHash string) error {
	return s.gorm.Model(&model.User{}).Where("id = ?", id).
		Updates(map[string]interface{}{"password_hash": passwordHash, "updated_at": time.Now()}).Error
}

func (s *DB) UpdateUserStatus(id, status string) error {
	return s.gorm.Model(&model.User{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "updated_at": time.Now()}).Error
}

func (s *DB) ListUsers(search string) ([]model.User, error) {
	var users []model.User
	q := s.gorm.Order("created_at DESC").Limit(200)
	if search != "" {
		pattern := "%" + search + "%"
		q = q.Where("username ILIKE ? OR email ILIKE ?", pattern, pattern)
	}
	err := q.Find(&users).Error
	return users, err
}

func (s *DB) LoadSettings() (model.AuthSettings, error) {
	settings := model.DefaultAuthSettings()
	var row model.AuthSettings
	err := s.gorm.Where("id = ?", 1).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	return row, nil
}

func (s *DB) SaveSettings(settings *model.AuthSettings) error {
	settings.ID = 1
	settings.UpdatedAt = time.Now()
	return s.gorm.Save(settings).Error
}

func (s *DB) SettingsJSON() ([]byte, model.AuthSettings, error) {
	settings, err := s.LoadSettings()
	if err != nil {
		return nil, settings, err
	}
	b, err := json.Marshal(settings)
	return b, settings, err
}

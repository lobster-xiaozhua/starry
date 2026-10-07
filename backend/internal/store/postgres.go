package store

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"login-system/backend/internal/model"
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

func (s *DB) AutoMigrate() error {
	// 扩展必须先于建表创建：knowledge_chunks 的 vector(384) 列依赖 pgvector。
	for _, stmt := range []string{
		"CREATE EXTENSION IF NOT EXISTS pg_trgm",
		"CREATE EXTENSION IF NOT EXISTS pgcrypto",
		"CREATE EXTENSION IF NOT EXISTS vector",
	} {
		if err := s.gorm.Exec(stmt).Error; err != nil {
			return err
		}
	}
	if err := s.gorm.AutoMigrate(
		&model.User{}, &model.AuthSettings{},
		&model.Note{}, &model.Tag{}, &model.NoteTag{},
		&model.NoteStableID{}, &model.Attachment{},
		&model.Conversation{}, &model.Message{},
		&model.KnowledgeDoc{}, &model.KnowledgeChunk{},
		&model.AgentTask{},
		&model.Board{}, &model.BoardColumn{}, &model.BoardTask{},
		&model.DriveFile{},
		&model.VaultKey{}, &model.VaultItem{},
	); err != nil {
		return err
	}
	for _, stmt := range []string{
		"ALTER TABLE notes ADD COLUMN IF NOT EXISTS fts tsvector GENERATED ALWAYS AS " +
			"(setweight(to_tsvector('simple', coalesce(title, '')), 'A') || " +
			"setweight(to_tsvector('simple', coalesce(body, '')), 'B')) STORED",
		"CREATE INDEX IF NOT EXISTS idx_notes_user ON notes(user_id, archived, updated_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_notes_fts ON notes USING GIN (fts)",
		"CREATE INDEX IF NOT EXISTS idx_notes_title_trgm ON notes USING GIN (title gin_trgm_ops)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_user_name ON tags(user_id, name)",
	} {
		if err := s.gorm.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
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

func (s *DB) SeedSettings() error {
	settings := model.DefaultAuthSettings()
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

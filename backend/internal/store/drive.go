package store

import (
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"login-system/backend/internal/model"
)

// DriveStore 负责网盘文件的磁盘读写，按用户分子目录，与笔记附件存储隔离。
type DriveStore struct {
	root string
}

func NewDriveStore(root string) *DriveStore {
	return &DriveStore{root: root}
}

func (d *DriveStore) Init() error {
	return os.MkdirAll(d.root, 0o755)
}

func (d *DriveStore) userDir(userID uuid.UUID) string {
	return filepath.Join(d.root, userID.String())
}

// Save 将 reader 内容写入用户目录，返回磁盘存储名（不含路径）。
func (d *DriveStore) Save(userID uuid.UUID, r io.Reader) (storedName string, err error) {
	dir := d.userDir(userID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	storedName = uuid.NewString()
	dst := filepath.Join(dir, storedName)
	fh, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	if _, err := io.Copy(fh, r); err != nil {
		os.Remove(dst)
		return "", err
	}
	return storedName, nil
}

// Path 返回磁盘文件路径。
func (d *DriveStore) Path(userID uuid.UUID, storedName string) string {
	return filepath.Join(d.userDir(userID), storedName)
}

// Delete 删除磁盘文件（忽略不存在）。
func (d *DriveStore) Delete(userID uuid.UUID, storedName string) error {
	if storedName == "" {
		return nil
	}
	err := os.Remove(d.Path(userID, storedName))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// ===== 网盘元数据（DriveFile 行） =====

// ListDriveChildren 列出某父目录下的条目（文件夹优先，再按名称排序）。
func (s *DB) ListDriveChildren(userID uuid.UUID, parentID *uuid.UUID) ([]model.DriveFile, error) {
	var rows []model.DriveFile
	q := s.gorm.Where("user_id = ?", userID)
	if parentID == nil {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", *parentID)
	}
	err := q.Order("is_dir DESC, name ASC").Find(&rows).Error
	return rows, err
}

// GetDriveFile 按用户与 ID 取条目（校验归属）。
func (s *DB) GetDriveFile(userID, id uuid.UUID) (*model.DriveFile, error) {
	var f model.DriveFile
	err := s.gorm.Where("user_id = ? AND id = ?", userID, id).First(&f).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// CreateDriveFolder 新建文件夹。
func (s *DB) CreateDriveFolder(userID uuid.UUID, parentID *uuid.UUID, name string) (*model.DriveFile, error) {
	f := model.DriveFile{
		ID:       uuid.New(),
		UserID:   userID,
		ParentID: parentID,
		Name:     name,
		IsDir:    true,
		Mime:     "directory",
	}
	if err := s.gorm.Create(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

// InsertDriveFile 写入文件元数据（磁盘落盘由调用方完成，传入 storedName）。
func (s *DB) InsertDriveFile(f *model.DriveFile) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return s.gorm.Create(f).Error
}

// RenameDriveFile 改名。
func (s *DB) RenameDriveFile(userID, id uuid.UUID, name string) error {
	return s.gorm.Model(&model.DriveFile{}).Where("user_id = ? AND id = ?", userID, id).
		Update("name", name).Error
}

// SumDriveUsage 统计某用户已用字节数（文件之和）。
func (s *DB) SumDriveUsage(userID uuid.UUID) (int64, error) {
	var total int64
	err := s.gorm.Model(&model.DriveFile{}).Where("user_id = ? AND is_dir = false", userID).
		Select("COALESCE(SUM(size),0)").Scan(&total).Error
	return total, err
}

// DeleteDriveFileRow 仅删除元数据行（磁盘清理由调用方处理）。
func (s *DB) DeleteDriveFileRow(userID, id uuid.UUID) error {
	return s.gorm.Where("user_id = ? AND id = ?", userID, id).Delete(&model.DriveFile{}).Error
}

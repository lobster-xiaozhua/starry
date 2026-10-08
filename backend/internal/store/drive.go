package store

import (
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"starry/backend/internal/model"
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

// ===== 网盘配额计数（drive_quota：每用户一行，已用空间的权威来源） =====

// EnsureDriveQuotaRow 惰性创建（并回填）某用户的配额行：
//   - 已存在则 ON CONFLICT 无操作；
//   - 不存在则一次性按已有文件体积回填，使 used 与文件行初始一致。
//
// 回填值来自子查询 SUM(size)，因此在并发首建时无论哪条 INSERT 胜出，
// 计算出的 used 都相同，不会产生回填竞态。
func (s *DB) EnsureDriveQuotaRow(userID uuid.UUID) error {
	return s.gorm.Exec(`
		INSERT INTO drive_quota (user_id, used, created_at, updated_at)
		VALUES (?, COALESCE((SELECT SUM(size) FROM drive_files WHERE user_id = ? AND is_dir = false), 0), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) DO NOTHING
	`, userID, userID).Error
}

// GetOrCreateDriveQuota 确保配额行存在并返回当前计数（供读取展示用）。
func (s *DB) GetOrCreateDriveQuota(userID uuid.UUID) (*model.DriveQuota, error) {
	if err := s.EnsureDriveQuotaRow(userID); err != nil {
		return nil, err
	}
	var q model.DriveQuota
	if err := s.gorm.Where("user_id = ?", userID).First(&q).Error; err != nil {
		return nil, err
	}
	return &q, nil
}

// ClaimDriveSpace 原子占用 size 字节：仅当 used+size <= quota 时更新成功，
// 返回是否占用成功。单条带条件 UPDATE 在数据库层面串行执行，从根本上杜绝
// 并发上传的「检查-使用」竞态（TOCTOU），永远不会让 used 越过 quota。
func (s *DB) ClaimDriveSpace(userID uuid.UUID, size, quota int64) (bool, error) {
	res := s.gorm.Exec(
		"UPDATE drive_quota SET used = used + ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ? AND used + ? <= ?",
		size, userID, size, quota,
	)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// ReleaseDriveSpace 归还 size 字节（删除文件时调用），下探到 0 不会变负。
func (s *DB) ReleaseDriveSpace(userID uuid.UUID, size int64) error {
	return s.gorm.Exec(
		"UPDATE drive_quota SET used = CASE WHEN used - ? < 0 THEN 0 ELSE used - ? END, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?",
		size, size, userID,
	).Error
}

// DeleteDriveFileAndReclaim 在单事务内删除文件元数据行并归还其占用空间，
// 保证「删行」与「回写配额」要么都完成、要么都不做，避免半删导致的配额漂移。
// 文件不存在（已被删）时安全返回，reclaimed 为实际归还的字节数（文件夹为 0）。
func (s *DB) DeleteDriveFileAndReclaim(userID, id uuid.UUID) (int64, error) {
	var reclaimed int64
	err := s.gorm.Transaction(func(tx *gorm.DB) error {
		var f model.DriveFile
		if err := tx.Where("user_id = ? AND id = ?", userID, id).First(&f).Error; err != nil {
			if isNotFound(err) {
				return nil
			}
			return err
		}
		if !f.IsDir {
			reclaimed = f.Size
		}
		if err := tx.Where("user_id = ? AND id = ?", userID, id).Delete(&model.DriveFile{}).Error; err != nil {
			return err
		}
		if reclaimed > 0 {
			if err := tx.Exec(
				"UPDATE drive_quota SET used = CASE WHEN used - ? < 0 THEN 0 ELSE used - ? END, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?",
				reclaimed, reclaimed, userID,
			).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return reclaimed, err
}

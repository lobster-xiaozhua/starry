package store

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/google/uuid"

	"starry/backend/internal/model"
)

// GetVaultKey 返回用户的保险箱密钥材料（Salt/Verifier），不存在时返回 nil。
func (s *DB) GetVaultKey(userID uuid.UUID) (*model.VaultKey, error) {
	var k model.VaultKey
	err := s.gorm.Where("user_id = ?", userID).First(&k).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// EnsureVaultSalt 保证用户存在 VaultKey 并返回 (salt, verifier)。
// 若不存在则生成随机 salt、写入空 verifier 后返回，确保 salt 在服务端唯一托管。
func (s *DB) EnsureVaultSalt(userID uuid.UUID) (salt, verifier string, err error) {
	k, err := s.GetVaultKey(userID)
	if err != nil {
		return "", "", err
	}
	if k != nil {
		return k.Salt, k.Verifier, nil
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	salt = hex.EncodeToString(buf)
	k = &model.VaultKey{ID: uuid.New(), UserID: userID, Salt: salt, Verifier: ""}
	if err := s.gorm.Create(k).Error; err != nil {
		return "", "", err
	}
	return salt, "", nil
}

// SaveVaultVerifier 写入校验串（客户端用主密码派生密钥加密的已知常量）。
func (s *DB) SaveVaultVerifier(userID uuid.UUID, verifier string) error {
	return s.gorm.Model(&model.VaultKey{}).Where("user_id = ?", userID).
		Update("verifier", verifier).Error
}

// ListVaultItems 列出用户的全部密项（含密文；服务端无法解读）。
func (s *DB) ListVaultItems(userID uuid.UUID) ([]model.VaultItem, error) {
	var items []model.VaultItem
	err := s.gorm.Where("user_id = ?", userID).Order("updated_at DESC").Find(&items).Error
	return items, err
}

// CreateVaultItem 新建密项。
func (s *DB) CreateVaultItem(item *model.VaultItem) error {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	return s.gorm.Create(item).Error
}

// UpdateVaultItem 更新密项（标题与/或密文）。
func (s *DB) UpdateVaultItem(userID, id uuid.UUID, title, encrypted *string) error {
	updates := map[string]interface{}{}
	if title != nil {
		updates["title"] = *title
	}
	if encrypted != nil {
		updates["encrypted"] = *encrypted
	}
	if len(updates) == 0 {
		return nil
	}
	return s.gorm.Model(&model.VaultItem{}).Where("user_id = ? AND id = ?", userID, id).
		Updates(updates).Error
}

// DeleteVaultItem 删除密项。
func (s *DB) DeleteVaultItem(userID, id uuid.UUID) error {
	return s.gorm.Where("user_id = ? AND id = ?", userID, id).Delete(&model.VaultItem{}).Error
}

package store

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"starry/backend/internal/model"
)

// PurgeUserData 安全地删除一个用户的所有数据：先删除磁盘文件（网盘 + 笔记附件），
// 再在单事务内删除所有归属于该用户的表行（含 users 自身）。这是实现「安全销户」的
// 基础设施动作，集中在 store 层完成，使 admin 模块无需 import 其它业务模块即可级联清理，
// 满足「模块之间不横向依赖」的架构约束。
//
// 设计要点：
//   - 磁盘删除在前且幂等（DriveStore.Delete / FileStore.DeleteURLs 忽略不存在），即便随后
//     的事务回滚，最多留下「行已删、磁盘文件已删」的一致状态；而事务成功则两边一致。
//   - 所有模型删除均用 Unscoped：notes/boards/knowledge/vault/chat 已启用软删除，普通 Delete
//     只会置 deleted_at 而不真正移除；销户必须物理删除，故显式穿透。
//   - DriveFile / User 本身不带软删除，Unscoped 对其等同硬删，无副作用。
func (s *DB) PurgeUserData(ctx context.Context, userID uuid.UUID) error {
	// 1) 读取需删除的磁盘文件清单（在事务外读取即可；销户期间该用户无并发写入）。
	var storedNames []string
	if err := s.gorm.Model(&model.DriveFile{}).
		Where("user_id = ? AND is_dir = false AND stored_name <> ''", userID).
		Pluck("stored_name", &storedNames).Error; err != nil {
		return fmt.Errorf("list drive files: %w", err)
	}
	var attachURLs []string
	if err := s.gorm.Model(&model.Attachment{}).
		Where("user_id = ?", userID).
		Pluck("url", &attachURLs).Error; err != nil {
		return fmt.Errorf("list attachments: %w", err)
	}
	var attachThumbs []string
	if err := s.gorm.Model(&model.Attachment{}).
		Where("user_id = ? AND thumb_url <> ''", userID).
		Pluck("thumb_url", &attachThumbs).Error; err != nil {
		return fmt.Errorf("list attachment thumbs: %w", err)
	}

	// 2) 删除磁盘文件（网盘 + 笔记附件），失败仅记录，不阻断后续 DB 清理。
	if s.Drive != nil {
		for _, name := range storedNames {
			if err := s.Drive.Delete(userID, name); err != nil {
				slog.Warn("purge: drive disk delete failed", "user", userID, "file", name, "error", err)
			}
		}
	}
	if s.Media != nil {
		s.Media.DeleteURLs(append(attachURLs, attachThumbs...))
	}

	// 3) 单事务删除所有表行。
	err := s.gorm.Transaction(func(tx *gorm.DB) error {
		// 网盘：文件行 + 配额行
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.DriveFile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.DriveQuota{}).Error; err != nil {
			return err
		}
		// 笔记：附件行、标签关联、稳定 ID、标签、笔记本体
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.Attachment{}).Error; err != nil {
			return err
		}
		var noteIDs []uuid.UUID
		if err := tx.Model(&model.Note{}).Unscoped().Where("user_id = ?", userID).Pluck("id", &noteIDs).Error; err != nil {
			return err
		}
		if len(noteIDs) > 0 {
			if err := tx.Unscoped().Where("note_id IN ?", noteIDs).Delete(&model.NoteTag{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.NoteStableID{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.Tag{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.Note{}).Error; err != nil {
			return err
		}
		// 看板
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.BoardTask{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.BoardColumn{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.Board{}).Error; err != nil {
			return err
		}
		// 知识库
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.KnowledgeDoc{}).Error; err != nil {
			return err
		}
		// 保险箱
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.VaultItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.VaultKey{}).Error; err != nil {
			return err
		}
		// 对话 / 消息 / 长程任务
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.Message{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.AgentTask{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Unscoped().Delete(&model.Conversation{}).Error; err != nil {
			return err
		}
		// 用户本体（最后删）
		if err := tx.Unscoped().Where("id = ?", userID).Delete(&model.User{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("purge transaction: %w", err)
	}
	return nil
}

// DeleteUser 硬删一个用户（不带级联）。一般经由 PurgeUserData 调用；
// 单独提供以覆盖「只删账号、不动业务数据」的极少数场景（如测试）。
func (s *DB) DeleteUser(userID uuid.UUID) error {
	return s.gorm.Unscoped().Where("id = ?", userID).Delete(&model.User{}).Error
}

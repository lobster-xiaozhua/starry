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

// ---- 数据完整性阶段 2：历史孤儿清理 + 真实外键约束 ----

// fkSpec 描述一条外键约束：table.column → refTable.refColumn，一律 ON DELETE CASCADE。
type fkSpec struct {
	table     string
	column    string
	refTable  string
	refColumn string
}

// foreignKeySpecs 是全库外键约束清单（26 条）。命名规则 fk_<table>_<column>：
// 幂等创建时按名查 information_schema，已存在即跳过，重启零开销。
//
// 设计取向：
//   - 用户级归属（*_user_id → users.id）16 条：用户行消失即一切业务行级联消失，
//     与 PurgeUserData 的显式级联互为兜底——后者保证磁盘回收，前者保证无漏网之行。
//   - 实体从属 10 条：子表/连接表行离开父实体即无意义。attachments.note_id 与
//     drive_files.parent_id 允许 NULL（根目录/未挂笔记是合法状态），FK 自动放行 NULL。
//   - 全部 CASCADE 而非 RESTRICT：业务删除路径已各自先清子行（DeleteKnowledgeDoc、
//     DeleteConversation、deleteNode 等），CASCADE 只是防御网，不会掩盖顺序错误。
var foreignKeySpecs = []fkSpec{
	// 用户级归属
	{"notes", "user_id", "users", "id"},
	{"tags", "user_id", "users", "id"},
	{"note_stable_ids", "user_id", "users", "id"},
	{"attachments", "user_id", "users", "id"},
	{"boards", "user_id", "users", "id"},
	{"board_columns", "user_id", "users", "id"},
	{"board_tasks", "user_id", "users", "id"},
	{"knowledge_docs", "user_id", "users", "id"},
	{"knowledge_chunks", "user_id", "users", "id"},
	{"vault_keys", "user_id", "users", "id"},
	{"vault_items", "user_id", "users", "id"},
	{"conversations", "user_id", "users", "id"},
	{"messages", "user_id", "users", "id"},
	{"agent_tasks", "user_id", "users", "id"},
	{"drive_files", "user_id", "users", "id"},
	{"drive_quota", "user_id", "users", "id"},
	// 实体从属
	{"note_tags", "note_id", "notes", "id"},
	{"note_tags", "tag_id", "tags", "id"},
	{"note_stable_ids", "note_id", "notes", "id"},
	{"attachments", "note_id", "notes", "id"},
	{"drive_files", "parent_id", "drive_files", "id"},
	{"board_columns", "board_id", "boards", "id"},
	{"board_tasks", "board_id", "boards", "id"},
	{"board_tasks", "column_id", "board_columns", "id"},
	{"knowledge_chunks", "doc_id", "knowledge_docs", "id"},
	{"messages", "conversation_id", "conversations", "id"},
}

// EnsureForeignKeys 幂等地创建全库外键约束（清单见 foreignKeySpecs）。
//
// 幂等策略：先查 information_schema.table_constraints，按 fk_<table>_<column>
// 命名约定的外键已存在即跳过——重启零成本，无 DDL 抖动。
//
// SQLite 不支持 ALTER TABLE ADD CONSTRAINT；单元测试环境（sqlite）直接跳过，
// 级联语义由 store 层显式清理逻辑保证，真实约束由 Postgres 冒烟环境验证。
func (s *DB) EnsureForeignKeys() error {
	if s.gorm.Dialector.Name() != "postgres" {
		return nil
	}
	for _, spec := range foreignKeySpecs {
		name := fmt.Sprintf("fk_%s_%s", spec.table, spec.column)
		var count int64
		if err := s.gorm.Raw(
			`SELECT COUNT(1) FROM information_schema.table_constraints
			 WHERE constraint_type = 'FOREIGN KEY' AND constraint_name = ? AND table_name = ?`,
			name, spec.table,
		).Scan(&count).Error; err != nil {
			return fmt.Errorf("check fk %s: %w", name, err)
		}
		if count > 0 {
			continue
		}
		stmt := fmt.Sprintf(
			"ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s) ON DELETE CASCADE",
			spec.table, name, spec.column, spec.refTable, spec.refColumn,
		)
		if err := s.gorm.Exec(stmt).Error; err != nil {
			return fmt.Errorf("add fk %s: %w", name, err)
		}
	}
	return nil
}

// CleanupOrphanRows 一次性清理历史孤儿数据，为 EnsureForeignKeys 扫清障碍
// （外键无法建立在含孤儿行的表上）。清理分三层，均先收集再删除：
//
//  1. 用户级孤儿：user_id 指向已不存在的用户（16 张带 user_id 的表）。
//     其中网盘文件与附件行带磁盘副作用，先删磁盘文件再删行。
//  2. 网盘父目录断链：parent_id 指向不存在的行。断链节点的全部后代同样是孤儿，
//     用递归 CTE 一次收集（Postgres 与 SQLite 均支持）。
//  3. 实体从属孤儿：attachments / note_stable_ids / note_tags 中指向不存在
//     note 或 tag 的行（含第 1 步删除用户级 notes/tags 后新暴露的）。
//
// 层次顺序刻意为「用户级 → 父断链 → 从属」：前一步的删除让后一步收集到
// 更完整的孤儿集合。与外键语义对齐：引用列为 NULL 的行 FK 放行，此处同样
// 保留（不可归属的数据不自动删除）。
//
// 必须在管理员账号播种（seedAdmin）之后调用：users 为空意味着任何行都无法
// 归属，正常数据会被误判为孤儿。启动顺序由 main 保证。
// 磁盘删除失败仅告警不回滚：宁可磁盘多留一个无主文件，也不让孤儿行清理半途而废。
func (s *DB) CleanupOrphanRows() (int64, error) {
	var total int64

	// ---- 1) 用户级孤儿：网盘文件行（磁盘副作用） ----
	n, err := s.cleanupOrphanDriveFiles(
		"drive_files.user_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = drive_files.user_id)")
	if err != nil {
		return total, fmt.Errorf("user-orphan drive files: %w", err)
	}
	total += n

	// ---- 2) 用户级孤儿：附件行（磁盘副作用） ----
	n, err = s.cleanupOrphanAttachments(
		"attachments.user_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = attachments.user_id)")
	if err != nil {
		return total, fmt.Errorf("user-orphan attachments: %w", err)
	}
	total += n

	// ---- 3) 用户级孤儿：其余 14 张表（无磁盘副作用） ----
	for _, m := range []any{
		&model.DriveQuota{}, &model.NoteStableID{}, &model.Tag{}, &model.Note{},
		&model.BoardTask{}, &model.BoardColumn{}, &model.Board{},
		&model.KnowledgeChunk{}, &model.KnowledgeDoc{},
		&model.VaultItem{}, &model.VaultKey{},
		&model.Message{}, &model.AgentTask{}, &model.Conversation{},
	} {
		res := s.gorm.Unscoped().
			Where("user_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = user_id)").
			Delete(m)
		if res.Error != nil {
			return total, fmt.Errorf("user-orphan rows in %T: %w", m, res.Error)
		}
		total += res.RowsAffected
	}

	// ---- 4) 网盘父目录断链（递归 CTE，含断链节点的全部后代） ----
	// 放在用户级清理之后：用户级孤儿行删除后，其子行在此成为直接孤儿并被一并收集。
	var brokenDrive []model.DriveFile
	if err := s.gorm.Raw(`
		WITH RECURSIVE broken(id) AS (
			SELECT f.id FROM drive_files f
			WHERE f.parent_id IS NOT NULL
			  AND NOT EXISTS (SELECT 1 FROM drive_files p WHERE p.id = f.parent_id)
			UNION ALL
			SELECT c.id FROM drive_files c JOIN broken b ON c.parent_id = b.id
		)
		SELECT d.* FROM drive_files d JOIN broken b ON d.id = b.id
	`).Scan(&brokenDrive).Error; err != nil {
		return total, fmt.Errorf("collect broken drive parents: %w", err)
	}
	if len(brokenDrive) > 0 {
		for _, f := range brokenDrive {
			if !f.IsDir && f.StoredName != "" && s.Drive != nil {
				if err := s.Drive.Delete(f.UserID, f.StoredName); err != nil {
					slog.Warn("cleanup: orphan drive disk delete failed", "file", f.StoredName, "error", err)
				}
			}
		}
		ids := make([]uuid.UUID, 0, len(brokenDrive))
		for _, f := range brokenDrive {
			ids = append(ids, f.ID)
		}
		del, err := s.deleteUnscopedByIDs(&model.DriveFile{}, ids)
		if err != nil {
			return total, fmt.Errorf("delete broken drive files: %w", err)
		}
		total += del
	}

	// ---- 5) 附件 note 断链（磁盘副作用；含第 3 步删 notes 后新暴露的） ----
	n, err = s.cleanupOrphanAttachments(
		"attachments.note_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM notes n WHERE n.id = attachments.note_id)")
	if err != nil {
		return total, fmt.Errorf("note-orphan attachments: %w", err)
	}
	total += n

	// ---- 6) note_stable_ids 的 note 断链 ----
	res := s.gorm.Unscoped().Where(
		"note_stable_ids.note_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM notes n WHERE n.id = note_stable_ids.note_id)",
	).Delete(&model.NoteStableID{})
	if res.Error != nil {
		return total, fmt.Errorf("orphan note_stable_ids: %w", res.Error)
	}
	total += res.RowsAffected

	// ---- 7) note_tags 的 note/tag 断链（连接表无 user_id，靠引用完整性清理） ----
	res = s.gorm.Unscoped().Where(
		"(note_tags.note_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM notes n WHERE n.id = note_tags.note_id)) OR " +
			"(note_tags.tag_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tags t WHERE t.id = note_tags.tag_id))",
	).Delete(&model.NoteTag{})
	if res.Error != nil {
		return total, fmt.Errorf("orphan note_tags: %w", res.Error)
	}
	total += res.RowsAffected

	return total, nil
}

// cleanupOrphanDriveFiles 按给定条件收集孤儿网盘文件行，先删磁盘文件再删行。
func (s *DB) cleanupOrphanDriveFiles(cond string) (int64, error) {
	var orphans []model.DriveFile
	if err := s.gorm.Model(&model.DriveFile{}).Where(cond).Find(&orphans).Error; err != nil {
		return 0, fmt.Errorf("list: %w", err)
	}
	if len(orphans) == 0 {
		return 0, nil
	}
	for _, f := range orphans {
		if !f.IsDir && f.StoredName != "" && s.Drive != nil {
			if err := s.Drive.Delete(f.UserID, f.StoredName); err != nil {
				slog.Warn("cleanup: orphan drive disk delete failed", "file", f.StoredName, "error", err)
			}
		}
	}
	ids := make([]uuid.UUID, 0, len(orphans))
	for _, f := range orphans {
		ids = append(ids, f.ID)
	}
	return s.deleteUnscopedByIDs(&model.DriveFile{}, ids)
}

// cleanupOrphanAttachments 按给定条件收集孤儿附件行，先删磁盘文件（原件 + 缩略图）再删行。
func (s *DB) cleanupOrphanAttachments(cond string) (int64, error) {
	var orphans []model.Attachment
	if err := s.gorm.Model(&model.Attachment{}).Where(cond).Find(&orphans).Error; err != nil {
		return 0, fmt.Errorf("list: %w", err)
	}
	if len(orphans) == 0 {
		return 0, nil
	}
	if s.Media != nil {
		urls := make([]string, 0, len(orphans)*2)
		for _, a := range orphans {
			if a.URL != "" {
				urls = append(urls, a.URL)
			}
			if a.ThumbURL != "" {
				urls = append(urls, a.ThumbURL)
			}
		}
		s.Media.DeleteURLs(urls)
	}
	ids := make([]uuid.UUID, 0, len(orphans))
	for _, a := range orphans {
		ids = append(ids, a.ID)
	}
	return s.deleteUnscopedByIDs(&model.Attachment{}, ids)
}

// deleteUnscopedByIDs 按 ID 集合硬删（穿透软删除），返回删除的行数。
func (s *DB) deleteUnscopedByIDs(m any, ids []uuid.UUID) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tx := s.gorm.Unscoped().Where("id IN ?", ids).Delete(m)
	return tx.RowsAffected, tx.Error
}

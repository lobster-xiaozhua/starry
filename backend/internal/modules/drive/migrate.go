package drive

import (
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立网盘模块拥有的表（文件/文件夹元数据）。
//
// 目录浏览是按「用户 + 父目录」取一层的固定查询，因此建复合索引而不是两个单列索引：
// 单列 user_id 在用户文件很多时选择度不足，父目录过滤还得回表再筛。
func Migrate(db *store.DB) error {
	if err := db.AutoMigrate(&model.DriveFile{}, &model.DriveQuota{}); err != nil {
		return err
	}
	return db.ExecDDL(
		"CREATE INDEX IF NOT EXISTS idx_drive_user_parent ON drive_files(user_id, parent_id)",
	)
}

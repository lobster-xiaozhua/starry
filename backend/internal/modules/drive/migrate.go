package drive

import (
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立网盘模块拥有的表（文件/文件夹元数据）。
func Migrate(db *store.DB) error {
	return db.AutoMigrate(&model.DriveFile{})
}

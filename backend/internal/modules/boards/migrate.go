package boards

import (
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立任务看板模块拥有的表：看板、列与任务。
func Migrate(db *store.DB) error {
	return db.AutoMigrate(&model.Board{}, &model.BoardColumn{}, &model.BoardTask{})
}

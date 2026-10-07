package knowledge

import (
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立知识库模块拥有的表。
//
// KnowledgeChunk 的 embedding 是 vector(384) 列，依赖 pgvector 扩展；
// 扩展由 store.EnsureExtensions 在所有模块迁移之前统一创建。
func Migrate(db *store.DB) error {
	return db.AutoMigrate(&model.KnowledgeDoc{}, &model.KnowledgeChunk{})
}

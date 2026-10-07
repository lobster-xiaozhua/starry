package knowledge

import (
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Migrate 建立知识库模块拥有的表。
//
// KnowledgeChunk 的 embedding 是 vector(384) 列，依赖 pgvector 扩展；
// 扩展由 store.EnsureExtensions 在所有模块迁移之前统一创建。
//
// 向量索引（hnsw）按「能建就建」处理：托管数据库的 pgvector 版本可能过旧不支持 hnsw，
// 这时退化为顺序扫描仍然正确，只是慢——不应因为可选索引把服务挡在启动阶段之外。
func Migrate(db *store.DB) error {
	if err := db.AutoMigrate(&model.KnowledgeDoc{}, &model.KnowledgeChunk{}); err != nil {
		return err
	}
	if err := db.ExecDDL(
		"CREATE INDEX IF NOT EXISTS idx_knowledge_docs_user_created ON knowledge_docs(user_id, created_at DESC)",
	); err != nil {
		return err
	}
	db.ExecDDLBestEffort(
		"CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_embedding ON knowledge_chunks USING hnsw (embedding vector_cosine_ops)",
	)
	return nil
}

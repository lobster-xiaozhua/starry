package store

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"login-system/backend/internal/model"
)

// SaveKnowledgeDoc 写入文档元数据。注意：分块与向量由调用方在事务外批量插入。
func (s *DB) SaveKnowledgeDoc(doc *model.KnowledgeDoc) error {
	return s.gorm.Create(doc).Error
}

// ListKnowledgeDocs 返回某用户的知识库文档列表（按时间倒序）。
func (s *DB) ListKnowledgeDocs(userID uuid.UUID) ([]model.KnowledgeDoc, error) {
	var docs []model.KnowledgeDoc
	err := s.gorm.Where("user_id = ?", userID).Order("created_at DESC").Find(&docs).Error
	return docs, err
}

// GetKnowledgeDoc 按用户与文档 ID 取文档。
func (s *DB) GetKnowledgeDoc(userID, docID uuid.UUID) (*model.KnowledgeDoc, error) {
	var doc model.KnowledgeDoc
	err := s.gorm.Where("user_id = ? AND id = ?", userID, docID).First(&doc).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// DeleteKnowledgeDoc 删除文档及其全部分块（级联清理向量）。
func (s *DB) DeleteKnowledgeDoc(userID, docID uuid.UUID) error {
	if err := s.gorm.Where("user_id = ? AND doc_id = ?", userID, docID).
		Delete(&model.KnowledgeChunk{}).Error; err != nil {
		return err
	}
	return s.gorm.Where("user_id = ? AND id = ?", userID, docID).
		Delete(&model.KnowledgeDoc{}).Error
}

// InsertChunks 批量写入文档分块（含向量字面量字符串，如 "[0.1,0.2,...]"）。
func (s *DB) InsertChunks(chunks []model.KnowledgeChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	return s.gorm.CreateInBatches(chunks, 100).Error
}

// SearchChunks 在用户知识库内做余弦相似度检索，返回最相似的 topK 分块。
// vectorStr 为查询向量的字面量字符串，pgvector 通过 ::vector 强转解析。
func (s *DB) SearchChunks(userID uuid.UUID, vectorStr string, topK int) ([]model.KnowledgeChunk, error) {
	if topK <= 0 || topK > 20 {
		topK = 5
	}
	var chunks []model.KnowledgeChunk
	err := s.gorm.Raw(`
		SELECT id, doc_id, user_id, content, created_at
		FROM knowledge_chunks
		WHERE user_id = ?
		ORDER BY embedding <=> ?::vector
		LIMIT ?
	`, userID, vectorStr, topK).Scan(&chunks).Error
	return chunks, err
}

// ChunkCount 统计某用户的分块总数（用于首页/概览）。
func (s *DB) ChunkCount(userID uuid.UUID) (int64, error) {
	var n int64
	err := s.gorm.Model(&model.KnowledgeChunk{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

func isNotFound(err error) bool {
	return err != nil && (err == gorm.ErrRecordNotFound ||
		err.Error() == "record not found")
}

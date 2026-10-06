package model

import (
	"time"

	"github.com/google/uuid"
)

// KnowledgeDoc 表示一份被纳入知识库的文档（用户上传的文本/网页/Markdown）。
// 一份文档会被切分为多条 KnowledgeChunk 并向量化存储。
type KnowledgeDoc struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID `gorm:"type:uuid;index" json:"-"`
	Title      string    `gorm:"size:512;not null" json:"title"`
	Source     string    `gorm:"size:1024" json:"source"`
	ChunkCount int       `json:"chunkCount"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (KnowledgeDoc) TableName() string { return "knowledge_docs" }

// KnowledgeChunk 是文档的一个切片，Embedding 以 pgvector 向量形式持久化，
// 检索时使用余弦距离（<=> 算子）做相似度排序。
type KnowledgeChunk struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	DocID     uuid.UUID `gorm:"type:uuid;index" json:"docId"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"-"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	Embedding string    `gorm:"type:vector(384)" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

func (KnowledgeChunk) TableName() string { return "knowledge_chunks" }

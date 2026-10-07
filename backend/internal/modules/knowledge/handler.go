package knowledge

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"starry/backend/internal/config"
	"starry/backend/internal/core"
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// Handler 提供企业知识库（RAG）的文档入库、检索、列举与删除。
// 向量化由独立的 embed 服务完成（本地免费模型），本 handler 负责存储与检索。
//
// embedder 是长生命周期的共享客户端（连接池 + 重试 + 分批），在 Register 时构造一次。
type Handler struct {
	db       *store.DB
	cfg      *config.Config
	embedder *embedClient
}

func New(db *store.DB, cfg *config.Config) *Handler {
	return &Handler{
		db:       db,
		cfg:      cfg,
		embedder: newEmbedClient(cfg.EmbedURL, cfg.AgentToken),
	}
}

func (h *Handler) userID(c *gin.Context) (uuid.UUID, bool) {
	id, _ := c.Get("userID")
	v, ok := id.(string)
	if !ok {
		return uuid.Nil, false
	}
	u, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, false
	}
	return u, true
}

// Ingest POST /api/knowledge/ingest
// 接收一段文本（标题+正文），切分为分块并向量化后入库。
func (h *Handler) Ingest(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	var in struct {
		Title   string `json:"title"`
		Content string `json:"content" binding:"required"`
		Source  string `json:"source"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		core.Fail(c, http.StatusBadRequest, 3008, "invalid body: "+err.Error())
		return
	}
	content := strings.TrimSpace(in.Content)
	if content == "" {
		core.Fail(c, http.StatusBadRequest, 3008, "content 不能为空")
		return
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = strings.Replace(content[:min(len(content), 40)], "\n", " ", -1)
	}

	chunks := chunkText(content, 400, 60)
	texts := make([]string, len(chunks))
	for i, ch := range chunks {
		texts[i] = ch
	}

	embeddings, err := h.embedder.Embed(c.Request.Context(), texts)
	if err != nil {
		// 向量化失败（嵌入服务不可用/模型未就绪）属于下游依赖故障，返回 503 而非 502/500，
		// 让前端能明确区分「本服务正常，但向量化模块暂不可用」，不影响对话等其余功能。
		ingestFailed(c, err)
		return
	}
	// 契约防御：向量与分块必须一一对应，否则宁可失败也不写半份数据。
	if len(embeddings) != len(chunks) {
		core.Fail(c, http.StatusInternalServerError, 3011,
			fmt.Sprintf("向量化结果数量异常: %d 个向量 / %d 个分块", len(embeddings), len(chunks)))
		return
	}

	doc := &model.KnowledgeDoc{
		ID:         uuid.New(),
		UserID:     uid,
		Title:      title,
		Source:     in.Source,
		ChunkCount: len(chunks),
		CreatedAt:  time.Now(),
	}
	if err := h.db.SaveKnowledgeDoc(doc); err != nil {
		core.Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	rows := make([]model.KnowledgeChunk, len(chunks))
	for i, emb := range embeddings {
		rows[i] = model.KnowledgeChunk{
			ID:        uuid.New(),
			DocID:     doc.ID,
			UserID:    uid,
			Content:   chunks[i],
			Embedding: vectorLiteral(emb),
			CreatedAt: time.Now(),
		}
	}
	if err := h.db.InsertChunks(rows); err != nil {
		// 入库失败回退删除文档元数据，避免悬挂
		_ = h.db.DeleteKnowledgeDoc(uid, doc.ID)
		core.Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	core.OK(c, gin.H{"docId": doc.ID.String(), "chunks": len(chunks)})
}

// Search GET /api/knowledge/search?q=...&k=5
// 对查询做向量化后在用户知识库内做相似度检索，返回片段（供 agent 的 knowledge_search 工具使用）。
func (h *Handler) Search(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		core.Fail(c, http.StatusBadRequest, 3008, "q 不能为空")
		return
	}
	k := 5
	if v := c.Query("k"); v != "" {
		if n, err := parseInt(v); err == nil && n > 0 {
			k = n
		}
	}
	embeddings, err := h.embedder.Embed(c.Request.Context(), []string{q})
	if err != nil {
		// 与入库保持一致的语义：检索依赖向量化，失败同样是「下游不可用」。
		ingestFailed(c, err)
		return
	}
	if len(embeddings) == 0 {
		core.Fail(c, http.StatusInternalServerError, 3011, "向量化结果为空")
		return
	}
	chunks, err := h.db.SearchChunks(uid, vectorLiteral(embeddings[0]), k)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	out := make([]gin.H, 0, len(chunks))
	for _, ch := range chunks {
		out = append(out, gin.H{"docId": ch.DocID.String(), "content": ch.Content})
	}
	core.OK(c, gin.H{"results": out})
}

// List GET /api/knowledge/docs
func (h *Handler) List(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	docs, err := h.db.ListKnowledgeDocs(uid)
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	core.OK(c, gin.H{"docs": docs})
}

// Delete DELETE /api/knowledge/docs/:id
func (h *Handler) Delete(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		core.Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		core.Fail(c, http.StatusBadRequest, 3004, "invalid doc id")
		return
	}
	if err := h.db.DeleteKnowledgeDoc(uid, docID); err != nil {
		core.Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	core.OK(c, gin.H{"id": docID.String()})
}

// ingestFailed 统一「向量化依赖不可用」的错误语义：返回 503 表示本服务正常、
// 下游暂时不可用且可重试，让前端与调用方能和其它 5xx 明确区分。
func ingestFailed(c *gin.Context, err error) {
	if ErrNotReady(err) {
		core.Fail(c, http.StatusServiceUnavailable, 3011, "向量化服务不可用: "+err.Error())
		return
	}
	core.Fail(c, http.StatusInternalServerError, 3011, "向量化失败: "+err.Error())
}

// chunkText 按字符窗口切分文本，带重叠以避免切断语义。
func chunkText(text string, size, overlap int) []string {
	runes := []rune(text)
	if len(runes) <= size {
		return []string{string(runes)}
	}
	var out []string
	step := size - overlap
	for i := 0; i < len(runes); i += step {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, strings.TrimSpace(string(runes[i:end])))
		if end == len(runes) {
			break
		}
	}
	return out
}

// vectorLiteral 将向量序列化为 pgvector 接受的字面量字符串 "[0.1,0.2,...]"。
func vectorLiteral(v []float64) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(x, 'f', -1, 64))
	}
	b.WriteByte(']')
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

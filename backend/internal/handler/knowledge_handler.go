package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"starry/backend/internal/config"
	"starry/backend/internal/model"
	"starry/backend/internal/store"
)

// KnowledgeHandler 提供企业知识库（RAG）的文档入库、检索、列举与删除。
// 向量化由 Node agent 服务完成（本地免费模型），本 handler 负责存储与检索。
type KnowledgeHandler struct {
	db  *store.DB
	cfg *config.Config
}

func NewKnowledgeHandler(db *store.DB, cfg *config.Config) *KnowledgeHandler {
	return &KnowledgeHandler{db: db, cfg: cfg}
}

func (h *KnowledgeHandler) userID(c *gin.Context) (uuid.UUID, bool) {
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
func (h *KnowledgeHandler) Ingest(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	var in struct {
		Title   string `json:"title"`
		Content string `json:"content" binding:"required"`
		Source  string `json:"source"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, 3008, "invalid body: "+err.Error())
		return
	}
	content := strings.TrimSpace(in.Content)
	if content == "" {
		Fail(c, http.StatusBadRequest, 3008, "content 不能为空")
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

	embeddings, err := h.embed(texts)
	if err != nil {
		Fail(c, http.StatusBadGateway, 3011, "向量化失败: "+err.Error())
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
		Fail(c, http.StatusInternalServerError, 3002, err.Error())
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
		Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	OK(c, gin.H{"docId": doc.ID.String(), "chunks": len(chunks)})
}

// Search GET /api/knowledge/search?q=...&k=5
// 对查询做向量化后在用户知识库内做相似度检索，返回片段（供 agent 的 knowledge_search 工具使用）。
func (h *KnowledgeHandler) Search(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		Fail(c, http.StatusBadRequest, 3008, "q 不能为空")
		return
	}
	k := 5
	if v := c.Query("k"); v != "" {
		if n, err := parseInt(v); err == nil && n > 0 {
			k = n
		}
	}
	embeddings, err := h.embed([]string{q})
	if err != nil {
		Fail(c, http.StatusBadGateway, 3011, "向量化失败: "+err.Error())
		return
	}
	chunks, err := h.db.SearchChunks(uid, vectorLiteral(embeddings[0]), k)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	out := make([]gin.H, 0, len(chunks))
	for _, ch := range chunks {
		out = append(out, gin.H{"docId": ch.DocID.String(), "content": ch.Content})
	}
	OK(c, gin.H{"results": out})
}

// List GET /api/knowledge/docs
func (h *KnowledgeHandler) List(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	docs, err := h.db.ListKnowledgeDocs(uid)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	OK(c, gin.H{"docs": docs})
}

// Delete DELETE /api/knowledge/docs/:id
func (h *KnowledgeHandler) Delete(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		Fail(c, http.StatusBadRequest, 3001, "invalid user")
		return
	}
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, http.StatusBadRequest, 3004, "invalid doc id")
		return
	}
	if err := h.db.DeleteKnowledgeDoc(uid, docID); err != nil {
		Fail(c, http.StatusInternalServerError, 3002, err.Error())
		return
	}
	OK(c, gin.H{"id": docID.String()})
}

// embed 调用 agent 服务的内部 /embed 端点批量获取向量（本地免费模型）。
func (h *KnowledgeHandler) embed(texts []string) ([][]float64, error) {
	body, _ := json.Marshal(gin.H{"texts": texts})
	req, err := http.NewRequest(http.MethodPost, h.cfg.AgentURL+"/api/agent/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.cfg.AgentToken != "" {
		req.Header.Set("X-Internal-Token", h.cfg.AgentToken)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, errFromStatus(resp.StatusCode, data)
	}
	var out struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out.Embeddings, nil
}

func errFromStatus(code int, data []byte) error {
	msg := string(data)
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return &embedError{code: code, msg: msg}
}

type embedError struct {
	code int
	msg  string
}

func (e *embedError) Error() string { return e.msg }

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

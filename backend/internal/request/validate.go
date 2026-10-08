// Package request 的「请求体内容校验」子集：与 query.go 的查询参数治理互补，
// 这里约束**请求体里的自由文本字段**——标题/名称/正文/密文等。
//
// 为什么单独放：MaxBodyBytes 只限制整体请求体大小（迭代 2），Sort 只限制 ORDER BY
// 白名单（迭代 3），但单个字段仍可被塞到接近 8MB。一个超大字段会直接冲击：
//   - 数据库列（超长字符串、PG 的 text 虽不限长但会膨胀索引与内存）；
//   - 下游算力（知识库 content 会被切分 + 向量化，一个 8MB 文档 = 上万次嵌入调用，DoS）；
//   - 存储体积（保险箱密文 blob）。
// 因此长度约束是入口治理闭环的最后一环。

package request

import "strings"

// 常见字段长度上限（按 rune 计，中文等多字节字符按「字符」而非「字节」算）。
const (
	// TitleMaxRunes 覆盖笔记/看板/会话/保险箱/网盘/知识库的标题与名称类字段。
	TitleMaxRunes = 200
	// ShortTextRunes 覆盖类型、优先级这类短枚举式文本。
	ShortTextRunes = 256
	// TaskNoteRunes 看板任务的备注。
	TaskNoteRunes = 8192
	// MessageRunes 单条对话消息正文（约 64KB），约束单条消息的存储与回放内存。
	MessageRunes = 65536
	// VaultBlobRunes 保险箱密文 / 主密码校验器（base64 编码，约 256KB）。
	VaultBlobRunes = 262144
	// KnowledgeRunes 单次知识库入库正文（约 100KB），约束分块数量与向量化次数。
	KnowledgeRunes = 102400
)

// TrimNonEmpty 返回去空格后的字符串，以及「是否非空」。
//
// 替代各 handler 里散落的 `strings.TrimSpace(x) == ""` 写法：统一「去空格 + 判空」
// 语义，既便于单测覆盖，也避免有人只判空不 trim 而漏掉纯空格这类脏输入。
// 对于必填字段，调用方应在结果为 false 时返回 400。
func TrimNonEmpty(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != ""
}

// WithinMaxRunes 判断字符串的 rune 长度是否不超过 max（中文等多字节字符按字符计）。
//
// 只负责「上限」，不负责判空（判空请用 TrimNonEmpty）。二者通常配合使用：
//
//	title, ok := request.TrimNonEmpty(body.Title)
//	if !ok || !request.WithinMaxRunes(title, request.TitleMaxRunes) {
//	    core.Fail(c, 400, xxx, "title 过长或为空")
//	    return
//	}
func WithinMaxRunes(s string, max int) bool {
	return len([]rune(s)) <= max
}

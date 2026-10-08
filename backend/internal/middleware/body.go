package middleware

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/core"
)

// bodyLimitStateKey 记录本次请求的「原始 Body + 当前生效的上限包装」，
// 供 RaiseBodyLimit 在 handler 内按需抬高额度（例如上传接口）。
const bodyLimitStateKey = "middleware:bodyLimitState"

// bodyLimitCode 是请求体超限时的业务码；与其它 4xxx 保持一致，便于前端归位提示。
const bodyLimitCode = 1009

// bodyLimitState 在中间件与 handler 之间传递体积限制的运行时状态。
// 之所以要持有 current 指针而不是只看请求体：handler 抬高上限后，
// 事后的超限判定必须跟着新的包装走，否则会漏判。
type bodyLimitState struct {
	orig    io.ReadCloser
	current *limitedBody
}

// limitedBody 在 MaxBytesReader 之上记录「是否真的撞了上限」。
//
// 光有 http.MaxBytesReader 不够：它只在 Read 返回一个 *http.MaxBytesError，
// 各 handler 的感知方式五花八门——ShouldBindJSON 会把它当成普通 400，
// io.ReadAll 则直接丢给调用方。统一在这里打标记，中间件才能在
// 「handler 没有产出任何响应」时兜底给出一个标准的 413。
type limitedBody struct {
	io.ReadCloser
	limit    int64
	overflow bool
}

func (b *limitedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if IsBodyTooLarge(err) {
		b.overflow = true
	}
	return n, err
}

// BodyLimit 给每个进入业务的请求加上读取上限。
//
// 为什么必须在中间件层做：handler 里的 ShouldBindJSON / io.ReadAll 都是「给多少读多少」，
// 单个超大 body 就能把实例内存吃满，而这类失败往往只剩一句 context canceled，
// 事后根本查不出是谁打的。
//
// 取值策略是「默认紧、按需松」：默认只覆盖 JSON 接口的合理量级，
// 上传类接口必须显式调用 RaiseBodyLimit 声明自己的额度并在注释里写明理由——
// 谁需要例外，谁就在代码里留下痕迹。
//
// 一个刻意的取舍：这里**没有**根据 Content-Length 提前拒绝。
// 提前拒绝看似更省（一个字节都不读），但它会在 handler 之前就下结论——
// 而上传接口的额度恰恰是在 handler 开头抬高的，于是合法的大文件会被误判为 413。
// 「先按头部判、再读」与「先进 handler 抬高额度、再读」在同一层无法共存，
// 因此这里以读取时限为准：它对 Content-Length 与 chunked 一视同仁，
// 代价是状态码不由中间件独占决定——需要精确 413 的入口请自行判定 IsBodyTooLarge。
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 || c.Request.Body == nil {
			c.Next()
			return
		}
		lb := &limitedBody{
			ReadCloser: http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes),
			limit:      maxBytes,
		}
		c.Set(bodyLimitStateKey, &bodyLimitState{orig: c.Request.Body, current: lb})
		c.Request.Body = lb

		c.Next()

		// 兜底：handler 已产出响应时以 handler 的判断为准；只有在它被超限错误提前打断、
		// 或者读完就静默返回时，才补一个标准的 413。chunked 请求（无 Content-Length）
		// 只能靠这条分支拦住。
		if st, ok := c.Get(bodyLimitStateKey); ok {
			if s := st.(*bodyLimitState); s.current.overflow && !c.Writer.Written() {
				FailBodyTooLarge(c, s.current.limit)
			}
		}
	}
}

// RaiseBodyLimit 在 handler 内抬高本次请求的体积上限，返回是否成功抬高。
//
// 返回 false 表示链路上没有安装 BodyLimit（例如某些单测直接构造 context）；
// 此时调用方应当自行处理，而不是假设上限已经生效。
func RaiseBodyLimit(c *gin.Context, maxBytes int64) bool {
	st, ok := c.Get(bodyLimitStateKey)
	if !ok {
		return false
	}
	s := st.(*bodyLimitState)
	if maxBytes <= 0 {
		return false
	}
	lb := &limitedBody{
		ReadCloser: http.MaxBytesReader(c.Writer, s.orig, maxBytes),
		limit:      maxBytes,
	}
	s.current = lb
	c.Request.Body = lb
	return true
}

// FailBodyTooLarge 输出统一格式的请求体超限响应。
func FailBodyTooLarge(c *gin.Context, limit int64) {
	core.Fail(c, http.StatusRequestEntityTooLarge, bodyLimitCode,
		"请求体超过上限 "+HumanBytes(limit))
}

// IsBodyTooLarge 判定错误是否来自请求体超限。
//
// http.MaxBytesReader 的溢出错误是导出的 *http.MaxBytesError，用 errors.As 判定
// 而不是匹配错误字符串：后者在标准库改文案的那一刻就会静默失效。
func IsBodyTooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}

// HumanBytes 把字节数格式化为易读的容量串（如 8 MiB）。限流以外它也用于业务侧的
// 「超过上限」提示：给用户的数字必须和实际配置同源，否则改了配置文案就失真。
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatInt(n/div, 10) + " " + string("KMGTP"[exp]) + "iB"
}

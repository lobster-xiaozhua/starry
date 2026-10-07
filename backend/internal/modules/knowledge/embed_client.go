package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"starry/backend/internal/logx"
)

// embedMaxBatch 与 embed 服务的 MAX_TEXTS 保持一致：超过该值的批次会被服务端静默截断。
// 这里按批拆分，保证「入多少文本，回多少向量」这条契约成立。
const embedMaxBatch = 64

// embedClient 封装对独立 embed 服务的调用。
//
// 设计要点（每一条都对应一次真实故障）：
//   - 共享 http.Client：原实现每次调用都 new 一个 Client，等价于每次新建连接池，
//     长文档入库时会瞬间产生大量短连接，端口与文件描述符都被吃掉。
//   - 绑定 context：客户端断开后不必再干等下游超时。
//   - 有限重试：仅对网络错误与 5xx/429 重试（请求幂等），4xx 立即失败。
//   - 结果数量校验：服务端返回数量不足时报错而不是让向量与文本错位。
type embedClient struct {
	endpoint string
	token    string
	http     *http.Client

	batch   int           // 单批上限，测试可调小
	retries int           // 最大重试次数
	backoff time.Duration // 重试退避基数
}

func newEmbedClient(endpoint, token string) *embedClient {
	return &embedClient{
		endpoint: strings.TrimSuffix(endpoint, "/") + "/api/agent/embed",
		token:    token,
		batch:    embedMaxBatch,
		retries:  2,
		backoff:  200 * time.Millisecond,
		http: &http.Client{
			// embed 首次调用要加载模型，给足整体超时；细粒度超时由 Transport 控制。
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:          16,
				MaxIdleConnsPerHost:   8,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				ExpectContinueTimeout: time.Second,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 5 * time.Second,
			},
		},
	}
}

// Embed 批量向量化，保证返回数量与入参相同且顺序一致。
func (c *embedClient) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([][]float64, 0, len(texts))
	for start := 0; start < len(texts); start += c.batch {
		end := start + c.batch
		if end > len(texts) {
			end = len(texts)
		}
		batch, err := c.embedBatch(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
	}
	return out, nil
}

func (c *embedClient) embedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	body, err := json.Marshal(map[string]any{"texts": texts})
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			if err := sleepCtx(ctx, c.backoff*time.Duration(attempt)); err != nil {
				return nil, err
			}
		}
		vecs, retryable, err := c.do(ctx, body)
		if err == nil {
			// 服务端截断会让后续分块与向量错位入库，宁可报错也不要脏数据。
			if len(vecs) != len(texts) {
				return nil, fmt.Errorf("embed 服务返回 %d 个向量，期望 %d 个", len(vecs), len(texts))
			}
			return vecs, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
		logx.L().Warn("embed 调用失败，准备重试",
			slog.String("endpoint", c.endpoint),
			slog.Int("attempt", attempt+1),
			slog.String("error", err.Error()))
	}
	return nil, lastErr
}

// do 执行一次请求；返回 (向量, 是否可重试, 错误)。
func (c *embedClient) do(ctx context.Context, body []byte) ([][]float64, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-Internal-Token", c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// context 取消/超时不算可重试：上游已经不等了。
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		return nil, true, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, isRetryableStatus(resp.StatusCode), &embedError{
			code: resp.StatusCode,
			msg:  truncate(string(data), 200),
		}
	}
	var out struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, false, err
	}
	return out.Embeddings, false, nil
}

// isRetryableStatus：5xx 与 429（限流）值得重试；其余 4xx 是请求本身的问题，重试无意义。
func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

type embedError struct {
	code int
	msg  string
}

func (e *embedError) Error() string {
	if e.msg == "" {
		return "embed service error " + http.StatusText(e.code)
	}
	return e.msg
}

func (e *embedError) Status() int { return e.code }

// ErrNotReady 判定下游是否处于「暂时不可用」（区别于配置错误等永久性失败），
// 供 handler 决定返回 503 还是 500。
func ErrNotReady(err error) bool {
	var ee *embedError
	if errors.As(err, &ee) {
		return true
	}
	return false
}

package knowledge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeEmbed 起一个假的 embed 服务；fn 决定每次请求如何应答。
// 同时记录收到的请求数，用于断言重试次数。
func fakeEmbed(t *testing.T, fn func(w http.ResponseWriter, texts []string, call int)) (*embedClient, *int, func()) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body struct {
			Texts []string `json:"texts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		calls++
		fn(w, body.Texts, calls)
	}))
	c := newEmbedClient(srv.URL, "tok")
	c.backoff = time.Millisecond // 测试不等真实退避
	return c, &calls, srv.Close
}

// replyVectors 用「文本内容」生成可识别的向量，便于断言顺序没有错位。
func replyVectors(w http.ResponseWriter, texts []string) {
	vecs := make([][]float64, len(texts))
	for i := range texts {
		vecs[i] = []float64{float64(i)}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": vecs})
}

// TestEmbedSplitsBatchesBeyondServerLimit 是截断数据回归测试：
// embed 服务单次最多处理 MAX_TEXTS 条，若客户端不自行分批，长文档入库会出现
// 「向量比文本少」——原来的实现把缺位填成零值 chunk 直接写库，造成脏数据。
func TestEmbedSplitsBatchesBeyondServerLimit(t *testing.T) {
	c, calls, done := fakeEmbed(t, func(w http.ResponseWriter, texts []string, _ int) {
		if len(texts) > 2 {
			t.Errorf("单批不应超过客户端上限, got %d", len(texts))
		}
		replyVectors(w, texts)
	})
	defer done()
	c.batch = 2

	texts := []string{"a", "b", "c", "d", "e"}
	got, err := c.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(got) != len(texts) {
		t.Fatalf("返回 %d 个向量, 期望 %d 个", len(got), len(texts))
	}
	if *calls != 3 {
		t.Fatalf("5 条文本按 2 分批应为 3 次请求, got %d", *calls)
	}
	// 每批的向量是批内下标，这里只校验数量与形状，顺序由下面的用例单独验证。
	for i, v := range got {
		if len(v) != 1 {
			t.Fatalf("第 %d 个向量形状异常: %v", i, v)
		}
	}
}

// TestEmbedKeepsOrderAcrossBatches 校验跨批拼接后顺序不错位。
func TestEmbedKeepsOrderAcrossBatches(t *testing.T) {
	c, _, done := fakeEmbed(t, func(w http.ResponseWriter, texts []string, _ int) {
		// 用「文本自身」编码进向量：vecs[i] = 该文本表示的整数
		vecs := make([][]float64, len(texts))
		for i, s := range texts {
			var n float64
			for _, r := range s {
				n = n*10 + float64(r-'0')
			}
			vecs[i] = []float64{n}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": vecs})
	})
	defer done()
	c.batch = 2

	got, err := c.Embed(context.Background(), []string{"1", "2", "3", "4", "5"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	want := []float64{1, 2, 3, 4, 5}
	for i, v := range got {
		if len(v) != 1 || v[0] != want[i] {
			t.Fatalf("第 %d 个向量 = %v, 期望 [%v]", i, v, want[i])
		}
	}
}

// TestEmbedRejectsShortResponse 保证服务端少给向量时报错，而不是静默错位入库。
func TestEmbedRejectsShortResponse(t *testing.T) {
	c, _, done := fakeEmbed(t, func(w http.ResponseWriter, texts []string, _ int) {
		truncated := texts[:1]
		replyVectors(w, truncated)
	})
	defer done()

	if _, err := c.Embed(context.Background(), []string{"a", "b", "c"}); err == nil {
		t.Fatalf("服务端截断时应返回错误，否则会产生错位数据")
	}
}

func TestEmbedRetriesOnTransientErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		{"500", http.StatusInternalServerError},
		{"503 模型未就绪", http.StatusServiceUnavailable},
		{"429 限流", http.StatusTooManyRequests},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, calls, done := fakeEmbed(t, func(w http.ResponseWriter, texts []string, call int) {
				if call == 1 {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte("boom"))
					return
				}
				replyVectors(w, texts)
			})
			defer done()

			got, err := c.Embed(context.Background(), []string{"a"})
			if err != nil {
				t.Fatalf("可重试错误应被重试消化: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("返回 %d 个向量", len(got))
			}
			if *calls != 2 {
				t.Fatalf("应重试一次后成功, got %d 次请求", *calls)
			}
		})
	}
}

func TestEmbedDoesNotRetryOnClientErrors(t *testing.T) {
	c, calls, done := fakeEmbed(t, func(w http.ResponseWriter, _ []string, _ int) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("internal token required"))
	})
	defer done()

	_, err := c.Embed(context.Background(), []string{"a"})
	if err == nil {
		t.Fatalf("4xx 应立即失败")
	}
	if *calls != 1 {
		t.Fatalf("4xx 不应重试, got %d 次请求", *calls)
	}
	if !strings.Contains(err.Error(), "internal token required") {
		t.Fatalf("错误信息应带上服务端原因, got %v", err)
	}
}

// TestEmbedHonorsContextCancel：客户端断开后不应继续占用下游连接。
func TestEmbedHonorsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		replyVectors(w, []string{"a"})
	}))
	defer srv.Close()
	c := newEmbedClient(srv.URL, "tok")
	c.backoff = time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Embed(ctx, []string{"a"}); err == nil {
		t.Fatalf("context 已取消时应立即失败")
	}
}

func TestEmbedDownUnreachableIsRetryableThenFails(t *testing.T) {
	// 指向一个没人监听的端口：连接失败应重试到上限后返回错误。
	c := newEmbedClient("http://127.0.0.1:1", "tok")
	c.backoff = time.Millisecond
	c.retries = 1
	if _, err := c.Embed(context.Background(), []string{"a"}); err == nil {
		t.Fatalf("下游不可达时应返回错误")
	}
}

func TestEmbedEmptyInputSkipsCall(t *testing.T) {
	c, calls, done := fakeEmbed(t, func(w http.ResponseWriter, texts []string, _ int) {
		replyVectors(w, texts)
	})
	defer done()

	got, err := c.Embed(context.Background(), nil)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(got) != 0 || *calls != 0 {
		t.Fatalf("空输入不应发起请求, got %d 次", *calls)
	}
}

func TestNewEmbedClientBuildsEndpoint(t *testing.T) {
	c := newEmbedClient("http://embed:3002/", "tok")
	if c.endpoint != "http://embed:3002/api/agent/embed" {
		t.Fatalf("endpoint = %q", c.endpoint)
	}
	if c.http.Timeout == 0 {
		t.Fatalf("必须设置整体超时，否则下游挂起会拖死请求")
	}
}

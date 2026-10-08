package request

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ctx 用给定查询串构造一个 gin 测试上下文，避免每个用例重复样板。
func ctx(query string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/"+query, nil)
	return c
}

func TestPage(t *testing.T) {
	cases := []struct {
		query string
		want  int
	}{
		{"", 1},          // 缺省
		{"?page=0", 1},   // 0 回落，绝不产生歧义
		{"?page=-3", 1},  // 负数回落
		{"?page=abc", 1}, // 非法回落
		{"?page=7", 7},   // 正常
	}
	for _, tc := range cases {
		if got := Page(ctx(tc.query)); got != tc.want {
			t.Fatalf("Page(%q)=%d，期望 %d", tc.query, got, tc.want)
		}
	}
}

func TestSize(t *testing.T) {
	cases := []struct {
		query    string
		def, max int
		want     int
	}{
		{"", 20, 100, 20},
		{"?size=0", 20, 100, 20},   // 0 回落默认
		{"?size=abc", 20, 100, 20}, // 非法回落默认
		{"?size=5", 20, 100, 5},
		{"?size=999", 20, 100, 100}, // 超上限钳制
	}
	for _, tc := range cases {
		if got := Size(ctx(tc.query), tc.def, tc.max); got != tc.want {
			t.Fatalf("Size(%q,%d,%d)=%d，期望 %d", tc.query, tc.def, tc.max, got, tc.want)
		}
	}
}

func TestIntParam(t *testing.T) {
	// 数量参数：给硬上限防止击穿下游。
	if got := IntParam(ctx("?k=10"), "k", 5, 1, 50); got != 10 {
		t.Fatalf("正常值应原样返回，实际 %d", got)
	}
	if got := IntParam(ctx("?k=999"), "k", 5, 1, 50); got != 50 {
		t.Fatalf("超上限应钳制到 50，实际 %d", got)
	}
	if got := IntParam(ctx("?k=-1"), "k", 5, 1, 50); got != 1 {
		t.Fatalf("低于下限应钳制到 1，实际 %d", got)
	}
	if got := IntParam(ctx("?k=abc"), "k", 5, 1, 50); got != 5 {
		t.Fatalf("非法值应回落默认 5，实际 %d", got)
	}
	if got := IntParam(ctx(""), "k", 5, 1, 50); got != 5 {
		t.Fatalf("缺省应回落默认 5，实际 %d", got)
	}
}

func TestSortWhitelist(t *testing.T) {
	allowed := map[string]string{
		"updated_at": "updated_at",
		"title":      "title",
	}
	def := "updated_at DESC"

	// 合法键 + 方向：原样映射
	if got := Sort(ctx("?sort=title&order=ASC"), allowed, def); got != "title ASC" {
		t.Fatalf("合法排序应映射为 title ASC，实际 %q", got)
	}
	// 合法键不传方向：默认 DESC
	if got := Sort(ctx("?sort=title"), allowed, def); got != "title DESC" {
		t.Fatalf("缺方向应回落 DESC，实际 %q", got)
	}
	// 非法键：回落默认
	if got := Sort(ctx("?sort=drop_table&order=ASC"), allowed, def); got != def {
		t.Fatalf("非法键应回落默认 %q，实际 %q", def, got)
	}
	// 非法方向：回落 DESC（不接受任意方向字符串，防注入）
	if got := Sort(ctx("?sort=title&order=DESC;DROP"), allowed, def); got != "title DESC" {
		t.Fatalf("非法方向应回落 DESC，实际 %q", got)
	}
	// 关键：用户输入绝不原样进 SQL
	evil := Sort(ctx("?sort=updated_at;DELETE FROM audit_logs--"), allowed, def)
	if evil != def {
		t.Fatalf("注入尝试必须被完全忽略并回落默认，实际 %q", evil)
	}
}

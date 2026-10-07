package knowledge

import (
	"strings"
	"testing"
)

// TestChunkText 验证文本切分：短文本整块返回，长文本按窗口+重叠切分。
func TestChunkText(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		size    int
		overlap int
		want    []string
	}{
		{
			name:    "短于窗口时整块返回",
			text:    "abc",
			size:    5,
			overlap: 2,
			want:    []string{"abc"},
		},
		{
			name:    "空文本返回单个空块",
			text:    "",
			size:    5,
			overlap: 2,
			want:    []string{""},
		},
		{
			name:    "长文本按步进切分并带重叠",
			text:    "abcdefghij",
			size:    5,
			overlap: 2,
			want:    []string{"abcde", "defgh", "ghij"},
		},
		{
			name:    "零重叠时首尾相接",
			text:    "abcdef",
			size:    2,
			overlap: 0,
			want:    []string{"ab", "cd", "ef"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := chunkText(tc.text, tc.size, tc.overlap)
			if len(got) != len(tc.want) {
				t.Fatalf("分块数不符: got %d (%v), want %d", len(got), got, len(tc.want))
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("第 %d 块不符: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestChunkTextCoversWholeText 确保切分不丢字符：所有块拼接后覆盖原文（考虑重叠）。
func TestChunkTextCoversWholeText(t *testing.T) {
	text := strings.Repeat("数据", 300) // 600 字符
	chunks := chunkText(text, 100, 20)
	joined := strings.Join(chunks, "")
	// 重叠会重复字符，因此拼接长度必然 >= 原文长度
	if len([]rune(joined)) < len([]rune(text)) {
		t.Fatalf("切分丢失内容: 拼接长度 %d < 原文 %d", len([]rune(joined)), len([]rune(text)))
	}
	// 首块应保留开头
	if !strings.HasPrefix(chunks[0], "数据") {
		t.Errorf("首块未保留开头: %q", chunks[0])
	}
}

// TestVectorLiteral 验证向量序列化为 pgvector 可接受的字面量。
func TestVectorLiteral(t *testing.T) {
	cases := []struct {
		name string
		vec  []float64
		want string
	}{
		{name: "空向量", vec: []float64{}, want: "[]"},
		{name: "整数", vec: []float64{1, 2, 3}, want: "[1,2,3]"},
		{name: "小数", vec: []float64{0.5, -1.25}, want: "[0.5,-1.25]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := vectorLiteral(tc.vec)
			if got != tc.want {
				t.Errorf("序列化不符: got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestVectorLiteralIsParseable 确保产出可被 pgvector 解析（方括号包裹、逗号分隔、无多余空格）。
func TestVectorLiteralIsParseable(t *testing.T) {
	got := vectorLiteral([]float64{0.1, 0.2, 0.3})
	if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
		t.Fatalf("缺少方括号包裹: %q", got)
	}
	body := strings.TrimSuffix(strings.TrimPrefix(got, "["), "]")
	if strings.Contains(body, " ") {
		t.Errorf("包含多余空格: %q", body)
	}
	if len(strings.Split(body, ",")) != 3 {
		t.Errorf("分量数不符: %q", body)
	}
}

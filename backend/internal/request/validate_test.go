package request

import "testing"

func TestTrimNonEmpty(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOk bool
	}{
		{"  hello  ", "hello", true},
		{"hello", "hello", true},
		{"", "", false},
		{"   ", "", false},
		{"\t\n", "", false},
		{"  中 文  ", "中 文", true},
	}
	for _, c := range cases {
		got, ok := TrimNonEmpty(c.in)
		if got != c.want || ok != c.wantOk {
			t.Errorf("TrimNonEmpty(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.wantOk)
		}
	}
}

func TestWithinMaxRunes(t *testing.T) {
	// 多字节正确性：一个中文字符 = 3 字节，但按 rune 计为 1。
	chinese := "中文测试" // 4 个 rune
	if !WithinMaxRunes(chinese, 4) {
		t.Errorf("4-rune chinese should fit within max=4")
	}
	if WithinMaxRunes(chinese, 3) {
		t.Errorf("4-rune chinese should NOT fit within max=3")
	}
	// ASCII
	if !WithinMaxRunes("abcd", 4) {
		t.Errorf("ascii boundary failed")
	}
	if WithinMaxRunes("abcde", 4) {
		t.Errorf("ascii overflow not detected")
	}
	// 空串与零上限
	if !WithinMaxRunes("", 0) {
		t.Errorf("empty string should always fit")
	}
	if !WithinMaxRunes("", 10) {
		t.Errorf("empty string should fit any max")
	}
}

func TestLengthConstantsAreSane(t *testing.T) {
	// 上限之间应保持合理的层级：标题最短，正文/密文允许更长。
	if TitleMaxRunes >= KnowledgeRunes {
		t.Errorf("TitleMaxRunes(%d) should be < KnowledgeRunes(%d)", TitleMaxRunes, KnowledgeRunes)
	}
	if KnowledgeRunes >= VaultBlobRunes {
		t.Errorf("KnowledgeRunes(%d) should be < VaultBlobRunes(%d)", KnowledgeRunes, VaultBlobRunes)
	}
	if MessageRunes <= 0 || TaskNoteRunes <= 0 {
		t.Errorf("content limits must be positive")
	}
}

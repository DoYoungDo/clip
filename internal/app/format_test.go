package app

import (
	"strings"
	"testing"
)

func TestHighlightTruncateMarksKeywordInShortText(t *testing.T) {
	got := highlightTruncate("我的密码是123", "密码", menuTextMaxWidth, defaultHighlightPrefix, defaultHighlightSuffix)
	want := "我的【密码】是123"
	if got != want {
		t.Fatalf("短文本应完整显示并包裹关键词, got %s, want %s", got, want)
	}
}

func TestHighlightTruncateKeepsKeywordVisibleInLongText(t *testing.T) {
	text := strings.Repeat("前", 100) + "密码" + strings.Repeat("后", 100)
	got := highlightTruncate(text, "密码", menuTextMaxWidth, defaultHighlightPrefix, defaultHighlightSuffix)
	if !strings.Contains(got, "【密码】") {
		t.Fatalf("长文本截断后关键词应保持可见, got %s", got)
	}
	if !strings.HasPrefix(got, "...") {
		t.Fatalf("长文本截断应有前导省略号, got %s", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("长文本截断应有尾部省略号, got %s", got)
	}
}

func TestHighlightTruncateKeywordAtStart(t *testing.T) {
	text := "密码" + strings.Repeat("字", 60)
	got := highlightTruncate(text, "密码", menuTextMaxWidth, defaultHighlightPrefix, defaultHighlightSuffix)
	if !strings.HasPrefix(got, "【密码】") {
		t.Fatalf("开头关键词应从首位开始标记, got %s", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("截断应有尾部省略号, got %s", got)
	}
}

func TestHighlightTruncateWithCustomMarks(t *testing.T) {
	got := highlightTruncate("我的密码是123", "密码", menuTextMaxWidth, ">>", "<<")
	want := "我的>>密码<<是123"
	if got != want {
		t.Fatalf("应支持自定义前后标记, got %s, want %s", got, want)
	}
}

func TestHighlightTruncateFallsBackWhenKeywordMissing(t *testing.T) {
	text := strings.Repeat("啊", 50)
	got := highlightTruncate(text, "密码", menuTextMaxWidth, defaultHighlightPrefix, defaultHighlightSuffix)
	want := truncateString(text, menuTextMaxWidth)
	if got != want {
		t.Fatalf("关键词不存在时应退回普通截断, got %s, want %s", got, want)
	}
}

func TestHighlightTruncateFallsBackWhenKeywordTooLong(t *testing.T) {
	got := highlightTruncate("短文本", "超长的关键词超长的关键词", 10, defaultHighlightPrefix, defaultHighlightSuffix)
	want := truncateString("短文本", 10)
	if got != want {
		t.Fatalf("关键词加标记超出长度时应退回普通截断, got %s, want %s", got, want)
	}
}

func TestHighlightTruncateFallsBackWhenKeywordEmpty(t *testing.T) {
	got := highlightTruncate("我的密码", "", menuTextMaxWidth, defaultHighlightPrefix, defaultHighlightSuffix)
	want := "我的密码"
	if got != want {
		t.Fatalf("关键词为空时应退回普通截断, got %s, want %s", got, want)
	}
}

func TestFormatMenuItemWithHighlightedText(t *testing.T) {
	item := NewClipItem(TypeText, []byte("我的密码是123"))
	got := formatMenuItemWithText(item, highlightTruncate(string(item.Content), "密码", menuTextMaxWidth, defaultHighlightPrefix, defaultHighlightSuffix), defaultTimeFormat)
	if !strings.Contains(got, "📝 [") {
		t.Fatalf("条目标题应保留前缀与时间格式, got %s", got)
	}
	if !strings.Contains(got, "我的【密码】是123") {
		t.Fatalf("条目标题应包含高亮文本, got %s", got)
	}
}

func TestToGoTimeLayout(t *testing.T) {
	cases := []struct {
		generic  string
		goLayout string
	}{
		{"hh:mm", "15:04"},
		{"YYYY-MM-DD HH:mm:ss", "2006-01-02 15:04:05"},
		{"YY/MM", "06/01"},
		{"DD日HH:mm", "02日15:04"},
		{"abc", "abc"},
	}
	for _, c := range cases {
		if got := toGoTimeLayout(c.generic); got != c.goLayout {
			t.Fatalf("通用格式转换错误: %s got %s, want %s", c.generic, got, c.goLayout)
		}
	}
}

func TestFormatMenuItemWithCustomTimeFormat(t *testing.T) {
	item := NewClipItem(TypeText, []byte("文本"))
	got := formatMenuItemWithText(item, itemDisplayText(item), "YYYY-MM-DD HH:mm:ss")
	if !strings.Contains(got, "["+item.Time.Format("2006-01-02 15:04:05")+"]") {
		t.Fatalf("条目标题应使用自定义时间格式, got %s", got)
	}
}

func TestTruncateStringByDisplayWidth(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		width int
		want  string
	}{
		{"半角恰好", strings.Repeat("a", 40), 40, strings.Repeat("a", 40)},
		{"半角超长", strings.Repeat("a", 41), 40, strings.Repeat("a", 40) + "..."},
		{"全角恰好", strings.Repeat("中", 20), 40, strings.Repeat("中", 20)},
		{"全角超长", strings.Repeat("中", 21), 40, strings.Repeat("中", 20) + "..."},
		{"混合恰好", strings.Repeat("中", 19) + "ab", 40, strings.Repeat("中", 19) + "ab"},
		{"混合超长", strings.Repeat("中", 19) + "abc", 40, strings.Repeat("中", 19) + "ab" + "..."},
	}
	for _, c := range cases {
		if got := truncateString(c.text, c.width); got != c.want {
			t.Fatalf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestHighlightTruncateAlignsDisplayWidth(t *testing.T) {
	text := strings.Repeat("a", 50) + "密码" + strings.Repeat("b", 50)
	got := highlightTruncate(text, "密码", menuTextMaxWidth, defaultHighlightPrefix, defaultHighlightSuffix)
	want := "..." + strings.Repeat("a", 18) + "【密码】" + strings.Repeat("b", 19) + "..."
	if got != want {
		t.Fatalf("混合宽度文本截断错误, got %q, want %q", got, want)
	}
}

package app

import (
	"crypto/md5"
	"fmt"
	"strings"
)

func formatMenuTitle(text string) string {
	lines := strings.Split(text, "\n")
	line := lines[0]
	return truncateString(line, 40)
}

const defaultHighlightPrefix = "【"

const defaultHighlightSuffix = "】"

const menuTextMaxWidth = 45

const defaultTimeFormat = "hh:mm"

var timeLayoutReplacer = strings.NewReplacer(
	"YYYY", "2006",
	"YY", "06",
	"MM", "01",
	"DD", "02",
	"HH", "15",
	"hh", "15",
	"mm", "04",
	"ss", "05",
)

func toGoTimeLayout(format string) string {
	return timeLayoutReplacer.Replace(format)
}

func formatMenuItem(item *ClipItem) string {
	return formatMenuItemWithText(item, itemDisplayText(item), defaultTimeFormat)
}

func itemDisplayText(item *ClipItem) string {
	switch item.Type {
	case TypeText:
		return truncateString(string(item.Content), menuTextMaxWidth)
	case TypeImage:
		label := "图片"
		if item.Name != "" {
			label = truncateString(item.Name, menuTextMaxWidth)
		}
		return fmt.Sprintf("%s [%s]", label, fmt.Sprintf("%x", md5.Sum(item.Content))[:8])
	}
	return ""
}

func formatMenuItemWithText(item *ClipItem, text string, timeFormat string) string {
	var prefix string
	switch item.Type {
	case TypeText:
		prefix = "📝"
	case TypeImage:
		prefix = "🖼️"
	}

	t := fmt.Sprintf("%s [%s]%s%s", prefix, item.Time.Format(toGoTimeLayout(timeFormat)), Ifel(item.From == FromRemote, " [R] ", ""), text)
	if t == "" {
		t = prefix + " [empty]"
	}

	return t
}

func highlightTruncate(text string, keyword string, maxWidth int, markPrefix string, markSuffix string) string {
	runes := []rune(text)
	kw := []rune(keyword)
	kwWidth := stringDisplayWidth(keyword)
	markPair := stringDisplayWidth(markPrefix) + stringDisplayWidth(markSuffix)
	if len(kw) == 0 || kwWidth+markPair > maxWidth {
		return truncateString(text, maxWidth)
	}

	idx := runeIndex(runes, kw)
	if idx < 0 {
		return truncateString(text, maxWidth)
	}

	markEnd := idx + len(kw)
	budget := maxWidth - kwWidth - markPair
	if budget < 0 {
		budget = 0
	}

	lead := budget / 2
	start := idx
	for start > 0 {
		w := runeDisplayWidth(runes[start-1])
		if lead < w {
			break
		}
		lead -= w
		start--
	}

	tail := budget - (budget/2 - lead)
	end := markEnd
	for end < len(runes) {
		w := runeDisplayWidth(runes[end])
		if tail < w {
			break
		}
		tail -= w
		end++
	}

	var b strings.Builder
	if start > 0 {
		b.WriteString("...")
	}
	b.WriteString(string(runes[start:idx]))
	b.WriteString(markPrefix)
	b.WriteString(string(runes[idx:markEnd]))
	b.WriteString(markSuffix)
	b.WriteString(string(runes[markEnd:end]))
	if end < len(runes) {
		b.WriteString("...")
	}
	return b.String()
}

func runeIndex(runes []rune, sub []rune) int {
	for i := 0; i+len(sub) <= len(runes); i++ {
		match := true
		for j, r := range sub {
			if runes[i+j] != r {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func formatMenuItemTooltip(item *ClipItem) string {
	switch item.Type {
	case TypeText:
		return string(item.Content)
	case TypeImage:
		if item.Name != "" {
			return item.Name
		}
		return "图片"
	default:
		return ""
	}
}

// truncateString 将字符串截断到最多 maxWidth 个显示宽度（中文等全角字符计 2，英文等半角字符计 1），
// 保证中文与英文条目的视觉长度一致。
func truncateString(s string, maxWidth int) string {
	width := 0
	for i, r := range s {
		width += runeDisplayWidth(r)
		if width > maxWidth {
			return s[:i] + "..."
		}
	}
	return s
}

// stringDisplayWidth 返回字符串的显示宽度（中文等全角字符计 2，英文等半角字符计 1）。
func stringDisplayWidth(s string) int {
	width := 0
	for _, r := range s {
		width += runeDisplayWidth(r)
	}
	return width
}

// runeDisplayWidth 返回单个字符的显示宽度：East Asian Wide / Fullwidth 记 2，其余记 1。
func runeDisplayWidth(r rune) int {
	if isWideRune(r) {
		return 2
	}
	return 1
}

// isWideRune 判断字符是否属于 East Asian Wide / Fullwidth 区间。
func isWideRune(r rune) bool {
	switch {
	case r < 0x1100:
		return false
	case r <= 0x115F, // Hangul Jamo
		r == 0x2329, r == 0x232A,
		r >= 0x2E80 && r <= 0xA4CF && r != 0x303F, // CJK 部首、汉字、日文、注音等
		r >= 0xAC00 && r <= 0xD7A3,                // Hangul Syllables
		r >= 0xF900 && r <= 0xFAFF,                // CJK Compatibility Ideographs
		r >= 0xFE10 && r <= 0xFE19,                // Vertical Forms
		r >= 0xFE30 && r <= 0xFE6F,                // CJK Compatibility Forms
		r >= 0xFF00 && r <= 0xFF60,                // Fullwidth Forms
		r >= 0xFFE0 && r <= 0xFFE6,                // Fullwidth Signs
		r >= 0x20000 && r <= 0x3FFFD:              // CJK Extension B+
		return true
	}
	return false
}

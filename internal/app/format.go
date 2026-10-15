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

const menuTextMaxRunes = 40

func formatMenuItem(item *ClipItem) string {
	return formatMenuItemWithText(item, itemDisplayText(item))
}

func itemDisplayText(item *ClipItem) string {
	switch item.Type {
	case TypeText:
		return truncateString(string(item.Content), menuTextMaxRunes)
	case TypeImage:
		label := "图片"
		if item.Name != "" {
			label = truncateString(item.Name, menuTextMaxRunes)
		}
		return fmt.Sprintf("%s [%s]", label, fmt.Sprintf("%x", md5.Sum(item.Content))[:8])
	}
	return ""
}

func formatMenuItemWithText(item *ClipItem, text string) string {
	var prefix string
	switch item.Type {
	case TypeText:
		prefix = "📝"
	case TypeImage:
		prefix = "🖼️"
	}

	t := fmt.Sprintf("%s [%s]%s%s", prefix, item.Time.Format("15:04"), Ifel(item.From == FromRemote, " [R] ", ""), text)
	if t == "" {
		t = prefix + " [empty]"
	}

	return t
}

func highlightTruncate(text string, keyword string, maxLen int, markPrefix string, markSuffix string) string {
	runes := []rune(text)
	kw := []rune(keyword)
	markPair := len([]rune(markPrefix)) + len([]rune(markSuffix))
	if len(kw) == 0 || len(kw)+markPair > maxLen {
		return truncateString(text, maxLen)
	}

	idx := runeIndex(runes, kw)
	if idx < 0 {
		return truncateString(text, maxLen)
	}

	markEnd := idx + len(kw)
	budget := maxLen - len(kw) - markPair
	if budget < 0 {
		budget = 0
	}
	lead := budget / 2
	if lead > idx {
		lead = idx
	}
	tail := budget - lead
	if markEnd+tail > len(runes) {
		tail = len(runes) - markEnd
	}

	var b strings.Builder
	if idx-lead > 0 {
		b.WriteString("...")
	}
	b.WriteString(string(runes[idx-lead : idx]))
	b.WriteString(markPrefix)
	b.WriteString(string(runes[idx:markEnd]))
	b.WriteString(markSuffix)
	b.WriteString(string(runes[markEnd : markEnd+tail]))
	if markEnd+tail < len(runes) {
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

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

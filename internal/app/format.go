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

func formatMenuItem(item *ClipItem) string {
	text := string(item.Content)
	var prefix string

	switch item.Type {
	case TypeText:
		prefix = "📝"
		text = truncateString(text, 40)
	case TypeImage:
		prefix = "🖼️"
		label := "图片"
		if item.Name != "" {
			label = truncateString(item.Name, 40)
		}
		text = fmt.Sprintf("%s [%s]", label, fmt.Sprintf("%x", md5.Sum(item.Content))[:8])
	}

	t := fmt.Sprintf("%s [%s]%s%s", prefix, item.Time.Format("15:04"), Ifel(item.From == FromRemote, " [R] ", ""), text)
	if t == "" {
		t = prefix + " [empty]"
	}

	return t
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

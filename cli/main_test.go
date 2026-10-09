package main

import (
	"strings"
	"testing"
)

func TestReadClipboardText(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"空输入", "", ""},
		{"仅换行", "\n", ""},
		{"尾随换行", "hello\n", "hello"},
		{"Windows 换行", "hello\r\n", "hello"},
		{"无换行", "hello", "hello"},
		{"多行保留内容", "line1\nline2\n", "line1\nline2"},
	}

	for _, tc := range cases {
		got, err := readClipboardText(strings.NewReader(tc.input))
		if err != nil {
			t.Fatalf("%s: 读取失败: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

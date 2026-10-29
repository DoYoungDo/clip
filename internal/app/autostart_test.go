package app

import (
	"strings"
	"testing"
)

func TestAutostartPlistContent(t *testing.T) {
	content := autostartPlistContent("/Applications/Clip.app/Contents/MacOS/Clip")
	if !strings.Contains(content, "<string>/Applications/Clip.app/Contents/MacOS/Clip</string>") {
		t.Fatalf("plist 应包含可执行文件路径: %s", content)
	}
	if !strings.Contains(content, "<string>"+autostartDarwinLabel+"</string>") {
		t.Fatalf("plist 应包含 Label: %s", content)
	}
	if !strings.Contains(content, "<key>RunAtLoad</key>") {
		t.Fatalf("plist 应包含 RunAtLoad: %s", content)
	}

	escaped := autostartPlistContent("/tmp/a&b<c>")
	if !strings.Contains(escaped, "/tmp/a&amp;b&lt;c&gt;") {
		t.Fatalf("plist 路径应做 XML 转义: %s", escaped)
	}
}

func TestAutostartDesktopContent(t *testing.T) {
	content := autostartDesktopContent("/usr/local/bin/clip")
	for _, want := range []string{"[Desktop Entry]", "Type=Application", "Name=" + appName, "Exec=/usr/local/bin/clip"} {
		if !strings.Contains(content, want) {
			t.Fatalf("desktop 应包含 %q: %s", want, content)
		}
	}

	quoted := autostartDesktopContent("/opt/My Apps/clip")
	if !strings.Contains(quoted, `Exec="/opt/My Apps/clip"`) {
		t.Fatalf("含空格路径应加引号: %s", quoted)
	}
}

package main

import (
	"testing"
	"time"
)

func TestEchoSuppressorSuppressesMatchingItemOnce(t *testing.T) {
	guard := &echoSuppressor{}
	remote := NewClipItemFromRemote(TypeImage, []byte("image-bytes"))
	local := NewClipItem(TypeImage, []byte("image-bytes"))

	guard.Mark(remote, time.Second)
	if !guard.ShouldSuppress(local) {
		t.Fatalf("应抑制刚刚回写到本地剪贴板的同内容图片")
	}
	if guard.ShouldSuppress(local) {
		t.Fatalf("同一条回声只应被抑制一次")
	}
}

func TestEchoSuppressorIgnoresExpiredOrDifferentItem(t *testing.T) {
	guard := &echoSuppressor{}
	remote := NewClipItemFromRemote(TypeImage, []byte("image-a"))
	different := NewClipItem(TypeImage, []byte("image-b"))

	guard.Mark(remote, 5*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	if guard.ShouldSuppress(remote.Clone()) {
		t.Fatalf("过期的回声标记不应继续生效")
	}

	guard.Mark(remote, time.Second)
	if guard.ShouldSuppress(different) {
		t.Fatalf("不同内容的图片不应被误抑制")
	}
}

func TestSelectClipboardChangePrefersChangedFormat(t *testing.T) {
	item, lastTextHash, lastImageHash := selectClipboardChange("", "", []byte("stale-text"), []byte("image-bytes"))
	if item == nil || item.Type != TypeImage {
		t.Fatalf("初次同时出现文本和图片时应优先图片")
	}

	item, _, _ = selectClipboardChange(lastTextHash, lastImageHash, []byte("fresh-text"), []byte("image-bytes"))
	if item == nil || item.Type != TypeText {
		t.Fatalf("只有文本发生变化时应选择文本")
	}

	item, _, _ = selectClipboardChange(lastTextHash, lastImageHash, []byte("stale-text"), []byte("image-bytes"))
	if item != nil {
		t.Fatalf("文本和图片都没变化时不应生成新记录")
	}
}

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
	item, lastTextHash, lastImageHash := selectClipboardChange("", "", []byte("stale-text"), []byte("image-bytes"), nil)
	if item == nil || item.Type != TypeImage {
		t.Fatalf("初次同时出现文本和图片时应优先图片")
	}

	item, _, _ = selectClipboardChange(lastTextHash, lastImageHash, []byte("fresh-text"), []byte("image-bytes"), nil)
	if item == nil || item.Type != TypeText {
		t.Fatalf("只有文本发生变化时应选择文本")
	}

	item, _, _ = selectClipboardChange(lastTextHash, lastImageHash, []byte("stale-text"), []byte("image-bytes"), nil)
	if item != nil {
		t.Fatalf("文本和图片都没变化时不应生成新记录")
	}
}

func TestSelectClipboardChangeReusesPendingWriteHash(t *testing.T) {
	imageBytes := []byte("image-bytes")
	pending := &ClipItem{Type: TypeImage, Content: imageBytes, Hash: "pending-hash"}

	item, textHash, imageHash := selectClipboardChange("", "", nil, imageBytes, pending)
	if item == nil || item.Type != TypeImage {
		t.Fatalf("应生成图片记录")
	}
	if item.Hash != "pending-hash" || imageHash != "pending-hash" || textHash != "" {
		t.Fatalf("应复用写回剪贴板时的已知哈希: item=%q image=%q", item.Hash, imageHash)
	}

	other := &ClipItem{Type: TypeImage, Content: []byte("other-bytes"), Hash: "pending-hash"}
	item, _, imageHash = selectClipboardChange("", "", nil, imageBytes, other)
	if item == nil {
		t.Fatalf("应生成图片记录")
	}
	if imageHash == "pending-hash" || item.Hash != calcClipItemHash(TypeImage, imageBytes) {
		t.Fatalf("内容不匹配时不应复用 pending 哈希: %q", imageHash)
	}
}

func TestMonitorCacheSkipsUnchangedContent(t *testing.T) {
	cache := &monitorCache{}

	if item := cache.poll([]byte("text-a"), nil, nil); item == nil || item.Type != TypeText {
		t.Fatalf("首次文本应生成记录")
	}
	if item := cache.poll([]byte("text-a"), nil, nil); item != nil {
		t.Fatalf("内容未变化时不应生成记录")
	}

	imageBytes := []byte("image-a")
	if item := cache.poll(nil, imageBytes, nil); item == nil || item.Type != TypeImage {
		t.Fatalf("图片变化时应生成记录")
	}
	if item := cache.poll(nil, imageBytes, nil); item != nil {
		t.Fatalf("图片未变化时不应生成记录")
	}

	if item := cache.poll([]byte("text-b"), nil, nil); item == nil || string(item.Content) != "text-b" {
		t.Fatalf("文本再次变化时应生成记录")
	}
}

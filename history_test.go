package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func resetTestLogChannel() {
	global_log_channel = make(chan LogEntry, 256)
}

func TestHistoryAddDedupAndMaxSize(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(2)

	first := NewClipItem(TypeText, []byte("one"))
	if !history.Add(first) {
		t.Fatalf("第一次添加应成功")
	}

	duplicate := NewClipItem(TypeText, []byte("one"))
	if history.Add(duplicate) {
		t.Fatalf("顶部重复项不应再次添加")
	}

	second := NewClipItem(TypeText, []byte("two"))
	third := NewClipItem(TypeText, []byte("three"))
	history.Add(second)
	history.Add(third)

	all := history.GetAll()
	if len(all) != 2 {
		t.Fatalf("期望最多保留 2 条记录，实际 %d", len(all))
	}
	if string(all[0].Content) != "three" || string(all[1].Content) != "two" {
		t.Fatalf("历史顺序不符合预期: got [%s, %s]", string(all[0].Content), string(all[1].Content))
	}
}

func TestHistoryDeleteAndClear(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(5)
	history.Add(NewClipItem(TypeText, []byte("one")))
	history.Add(NewClipItem(TypeText, []byte("two")))
	history.Add(NewClipItem(TypeText, []byte("three")))

	history.Delete(1)
	all := history.GetAll()
	if len(all) != 2 {
		t.Fatalf("删除后期望剩余 2 条，实际 %d", len(all))
	}
	if string(all[0].Content) != "three" || string(all[1].Content) != "one" {
		t.Fatalf("删除结果不正确: got [%s, %s]", string(all[0].Content), string(all[1].Content))
	}

	history.Clear()
	if got := len(history.GetAll()); got != 0 {
		t.Fatalf("清空后应无记录，实际 %d", got)
	}
}

func TestClipItemCloneIndependence(t *testing.T) {
	item := NewClipItem(TypeText, []byte("origin"))
	clone := item.Clone()
	remote := item.CloneToRemote()

	clone.Content[0] = 'X'
	remote.Content[0] = 'Y'

	if string(item.Content) != "origin" {
		t.Fatalf("修改克隆不应影响原始内容，实际 %s", string(item.Content))
	}
	if remote.From != FromRemote {
		t.Fatalf("远端克隆应标记为 FromRemote")
	}
}

func TestImageHashStableAcrossPNGEncoding(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	img.Set(1, 0, color.NRGBA{R: 0, G: 255, B: 0, A: 255})
	img.Set(0, 1, color.NRGBA{R: 0, G: 0, B: 255, A: 255})
	img.Set(1, 1, color.NRGBA{R: 255, G: 255, B: 0, A: 255})

	var fast bytes.Buffer
	encoderFast := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoderFast.Encode(&fast, img); err != nil {
		t.Fatalf("快速编码图片失败: %v", err)
	}

	var small bytes.Buffer
	encoderSmall := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoderSmall.Encode(&small, img); err != nil {
		t.Fatalf("高压缩编码图片失败: %v", err)
	}

	itemFast := NewClipItem(TypeImage, fast.Bytes())
	itemSmall := NewClipItem(TypeImage, small.Bytes())
	if itemFast.Hash != itemSmall.Hash {
		t.Fatalf("相同像素图片应生成相同稳定哈希: %s != %s", itemFast.Hash, itemSmall.Hash)
	}

	history := NewHistory(5)
	if !history.Add(itemFast) {
		t.Fatalf("第一次图片添加应成功")
	}
	if history.Add(itemSmall) {
		t.Fatalf("相同像素图片应被历史去重")
	}
}

func TestSyncLatestHistoryToGroupAddsTopItem(t *testing.T) {
	history := NewHistory(10)
	group := NewGroup("工作", false, 10)

	older := NewClipItem(TypeText, []byte("older"))
	latest := NewClipItem(TypeText, []byte("latest"))
	history.Add(older)
	history.Add(latest)

	if !syncLatestHistoryToGroup(history, group) {
		t.Fatalf("应将最近一条历史记录同步到分组")
	}

	top := group.History.GetTop()
	if top == nil || string(top.Content) != "latest" {
		t.Fatalf("分组顶部记录应为 latest，实际为 %#v", top)
	}
}

func TestSyncLatestHistoryToGroupHandlesEmptyAndDuplicate(t *testing.T) {
	group := NewGroup("工作", false, 10)

	if syncLatestHistoryToGroup(NewHistory(10), group) {
		t.Fatalf("空历史记录不应同步成功")
	}

	history := NewHistory(10)
	item := NewClipItem(TypeText, []byte("latest"))
	history.Add(item)

	if !syncLatestHistoryToGroup(history, group) {
		t.Fatalf("首次同步应成功")
	}
	if syncLatestHistoryToGroup(history, group) {
		t.Fatalf("重复同步同一条记录不应再次添加")
	}
}

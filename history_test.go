package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"
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

	all := history.GetAll()
	if !history.Delete(all[1]) {
		t.Fatalf("按指针删除应成功")
	}
	all = history.GetAll()
	if len(all) != 2 {
		t.Fatalf("删除后期望剩余 2 条，实际 %d", len(all))
	}
	if string(all[0].Content) != "three" || string(all[1].Content) != "one" {
		t.Fatalf("删除结果不正确: got [%s, %s]", string(all[0].Content), string(all[1].Content))
	}

	if history.Delete(nil) {
		t.Fatalf("删除 nil 应返回 false")
	}
	if history.Delete(NewClipItem(TypeText, []byte("ghost"))) {
		t.Fatalf("删除不存在的项应返回 false")
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

func TestHistoryConcurrentAddDelete(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(100)

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				item := NewClipItem(TypeText, []byte(fmt.Sprintf("g%d-%d", g, i)))
				history.Add(item)
				all := history.GetAll()
				if len(all) > 0 {
					history.Delete(all[len(all)-1])
				}
			}
		}(g)
	}
	wg.Wait()

	if got := len(history.GetAll()); got > 100 {
		t.Fatalf("并发操作后历史条数超出上限: %d", got)
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

func TestHistoryAddKeepLatestRemovesOlderDuplicates(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(10)

	history.Add(NewClipItem(TypeText, []byte("other")))
	history.Add(NewClipItem(TypeText, []byte("same")))

	if !history.AddKeepLatest(NewClipItem(TypeText, []byte("same"))) {
		t.Fatalf("相同内容再次添加应成功并置顶")
	}

	all := history.GetAll()
	if len(all) != 2 {
		t.Fatalf("期望保留 2 条，实际 %d", len(all))
	}
	if string(all[0].Content) != "same" || string(all[1].Content) != "other" {
		t.Fatalf("相同内容应只保留最近一条: got [%s, %s]", string(all[0].Content), string(all[1].Content))
	}
}

func TestHistoryAddKeepLatestDedupesImages(t *testing.T) {
	resetTestLogChannel()
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

	history := NewHistory(10)
	history.Add(NewClipItem(TypeImage, fast.Bytes()))

	latest := NewClipItem(TypeImage, small.Bytes())
	if !history.AddKeepLatest(latest) {
		t.Fatalf("相同像素图片再次添加应成功")
	}

	all := history.GetAll()
	if len(all) != 1 {
		t.Fatalf("相同像素图片应只保留一条，实际 %d", len(all))
	}
	if all[0] != latest {
		t.Fatalf("应保留最近添加的图片")
	}
}

func TestHistoryRemoveDuplicatesKeepsNewest(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(10)

	older := NewClipItem(TypeText, []byte("dup"))
	other := NewClipItem(TypeText, []byte("other"))
	newer := NewClipItem(TypeText, []byte("dup"))

	history.Add(older)
	history.Add(other)
	history.Add(newer)

	if removed := history.RemoveDuplicates(); removed != 1 {
		t.Fatalf("应移除 1 条重复记录，实际 %d", removed)
	}

	all := history.GetAll()
	if len(all) != 2 {
		t.Fatalf("去重后期望 2 条，实际 %d", len(all))
	}
	if all[0] != newer || all[1] != other {
		t.Fatalf("应保留最新的重复项")
	}
	if removed := history.RemoveDuplicates(); removed != 0 {
		t.Fatalf("再次去重不应移除记录，实际 %d", removed)
	}
}

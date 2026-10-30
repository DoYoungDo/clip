package app

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"
	"testing"
	"time"
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

func referenceImageHash(t *testing.T, content []byte) string {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("参考实现解码失败: %v", err)
	}

	bounds := img.Bounds()
	hasher := md5.New()
	_, _ = fmt.Fprintf(hasher, "%d:%d|", bounds.Dx(), bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			_, _ = fmt.Fprintf(hasher, "%04x%04x%04x%04x", r, g, b, a)
		}
	}
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func newGradientPNG(t testing.TB, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("编码图片失败: %v", err)
	}
	return buf.Bytes()
}

func TestCalcClipItemHashImageMatchesReference(t *testing.T) {
	pixels := []color.NRGBA{
		{R: 0, G: 0, B: 0, A: 0},
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 0, B: 0, A: 255},
		{R: 0, G: 255, B: 0, A: 128},
		{R: 0, G: 0, B: 255, A: 1},
		{R: 1, G: 2, B: 3, A: 4},
		{R: 254, G: 253, B: 252, A: 251},
		{R: 16, G: 32, B: 48, A: 64},
	}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for i := 0; i < 16; i++ {
		img.Set(i%4, i/4, pixels[i%len(pixels)])
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("编码图片失败: %v", err)
	}
	content := buf.Bytes()

	got := calcClipItemHash(TypeImage, content)
	want := referenceImageHash(t, content)
	if got != want {
		t.Fatalf("图片哈希与参考实现不一致: got %s want %s", got, want)
	}
}

func BenchmarkCalcClipItemHashImage(b *testing.B) {
	data := newGradientPNG(b, 1920, 1080)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calcClipItemHash(TypeImage, data)
	}
}

func TestHistorySetMaxSizeTruncatesAndLogs(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(10)
	for i := 0; i < 5; i++ {
		history.Add(NewClipItem(TypeText, []byte(fmt.Sprintf("item-%d", i))))
	}

	history.SetMaxSize(2)
	if got := len(history.GetAll()); got != 2 {
		t.Fatalf("期望截断为 2 条，实际 %d", got)
	}

	select {
	case <-global_log_channel:
	default:
		t.Fatalf("截断时应记录日志")
	}
}

func TestHistoryRenameReplacesItem(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(10)
	item := NewClipItem(TypeImage, []byte("image"))
	history.Add(item)

	if !history.Rename(item, "截图") {
		t.Fatalf("重命名应成功")
	}

	top := history.GetTop()
	if top == item {
		t.Fatalf("重命名应替换为新指针以避免并发读写")
	}
	if top.Name != "截图" {
		t.Fatalf("名称未更新: %q", top.Name)
	}
	if top.Hash != item.Hash || string(top.Content) != "image" {
		t.Fatalf("内容与哈希应保留")
	}
	if history.Rename(item, "again") {
		t.Fatalf("对已被替换的旧指针操作应失败")
	}
	if history.Rename(nil, "x") {
		t.Fatalf("nil 项应返回 false")
	}
}

func TestClipItemRefStableAcrossClone(t *testing.T) {
	item := NewClipItem(TypeText, []byte("hello"))
	ref := item.Ref()
	if ref == "" {
		t.Fatalf("Ref 不应为空")
	}
	if item.Clone().Ref() != ref {
		t.Fatalf("Clone 后 Ref 应保持一致")
	}
	if item.CloneToRemote().Ref() != ref {
		t.Fatalf("CloneToRemote 后 Ref 应保持一致")
	}
}

func TestHistoryFindByRef(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(10)
	item := NewClipItem(TypeText, []byte("hello"))
	history.Add(item)

	if got := history.FindByRef(item.Ref()); got != item {
		t.Fatalf("FindByRef 应返回目标条目")
	}
	if got := history.FindByRef("0:missing:1"); got != nil {
		t.Fatalf("不存在的引用应返回 nil")
	}
	if got := history.FindByRef(""); got != nil {
		t.Fatalf("空引用应返回 nil")
	}
}

func TestHistoryDeleteByRef(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(10)

	first := NewClipItem(TypeText, []byte("x"))
	first.Time = first.Time.Add(-2 * time.Second)
	other := NewClipItem(TypeText, []byte("y"))
	other.Time = other.Time.Add(-1 * time.Second)
	second := NewClipItem(TypeText, []byte("x"))
	history.Add(first)
	history.Add(other)
	history.Add(second)

	if first.Ref() == second.Ref() {
		t.Fatalf("同内容不同时间的条目 Ref 应不同")
	}

	changes := 0
	history.SetOnChange(func() { changes++ })

	if !history.DeleteByRef(first.Ref()) {
		t.Fatalf("按引用删除应成功")
	}
	if history.FindByRef(first.Ref()) != nil {
		t.Fatalf("目标条目应被删除")
	}
	if history.FindByRef(second.Ref()) == nil {
		t.Fatalf("相同内容的其他条目应保留")
	}
	if changes != 1 {
		t.Fatalf("删除应触发一次变更回调, got %d", changes)
	}

	if history.DeleteByRef(first.Ref()) {
		t.Fatalf("重复删除应返回 false")
	}
	if history.DeleteByRef("") {
		t.Fatalf("空引用应返回 false")
	}
	if changes != 1 {
		t.Fatalf("失败删除不应触发变更回调, got %d", changes)
	}
}

func TestHistoryRenameByRef(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(10)
	item := NewClipItem(TypeImage, []byte("image"))
	history.Add(item)

	changes := 0
	history.SetOnChange(func() { changes++ })

	if !history.RenameByRef(item.Ref(), "截图") {
		t.Fatalf("按引用重命名应成功")
	}

	renamed := history.FindByRef(item.Ref())
	if renamed == nil || renamed.Name != "截图" {
		t.Fatalf("重命名后名称错误: %#v", renamed)
	}
	if renamed == item {
		t.Fatalf("重命名应替换为新指针以避免并发读写")
	}
	if changes != 1 {
		t.Fatalf("重命名应触发一次变更回调, got %d", changes)
	}

	if history.RenameByRef("0:missing:1", "x") {
		t.Fatalf("不存在的引用重命名应返回 false")
	}
	if history.RenameByRef("", "x") {
		t.Fatalf("空引用重命名应返回 false")
	}
	if changes != 1 {
		t.Fatalf("失败重命名不应触发变更回调, got %d", changes)
	}
}

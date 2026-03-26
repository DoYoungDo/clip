package main

import "testing"

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

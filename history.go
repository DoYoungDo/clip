package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"sync"
	"time"
)

type ItemType int

const (
	TypeText ItemType = iota
	TypeImage
)

type ItemFrom int

const (
	FromLocal ItemFrom = iota
	FromRemote
)

type ClipItem struct {
	Type    ItemType  `json:"type"`
	Content []byte    `json:"content"`
	Hash    string    `json:"hash"`
	Time    time.Time `json:"time"`
	From    ItemFrom  `json:"from"`
}

func calcClipItemHash(itemType ItemType, content []byte) string {
	if itemType != TypeImage {
		return fmt.Sprintf("%x", md5.Sum(content))
	}

	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return fmt.Sprintf("%x", md5.Sum(content))
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

func NewClipItem(itemType ItemType, content []byte) *ClipItem {
	return &ClipItem{
		Type:    itemType,
		Content: append([]byte{}, content...),
		Hash:    calcClipItemHash(itemType, content),
		Time:    time.Now(),
		From:    FromLocal,
	}
}

func NewClipItemFromRemote(itemType ItemType, content []byte) *ClipItem {
	return &ClipItem{
		Type:    itemType,
		Content: append([]byte{}, content...),
		Hash:    calcClipItemHash(itemType, content),
		Time:    time.Now(),
		From:    FromRemote,
	}
}

func (c *ClipItem) CloneToRemote() *ClipItem {
	return &ClipItem{
		Type:    c.Type,
		Content: append([]byte{}, c.Content...),
		Hash:    c.Hash,
		Time:    c.Time,
		From:    FromRemote,
	}
}

func (c *ClipItem) Clone() *ClipItem {
	return &ClipItem{
		Type:    c.Type,
		Content: append([]byte{}, c.Content...),
		Hash:    c.Hash,
		Time:    c.Time,
		From:    c.From,
	}
}

type History struct {
	items   []*ClipItem
	maxSize uint
	mu      sync.RWMutex
}

func NewHistory(maxSize uint) *History {
	return &History{
		items:   []*ClipItem{},
		maxSize: maxSize,
	}
}

func (h *History) Add(item *ClipItem) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.items) > 0 {
		top := h.items[0]
		if top != nil && top.Type == item.Type && top.Hash == item.Hash {
			return false
		}
	}

	// 允许重复，直接添加到最前面
	h.items = append([]*ClipItem{item}, h.items...)
	if (uint)(len(h.items)) > h.maxSize {
		h.items = h.items[:h.maxSize]
	}
	return true
}

func (h *History) GetAll() []*ClipItem {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*ClipItem, len(h.items))
	copy(result, h.items)
	return result
}

func (h *History) GetTop() *ClipItem {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.items) > 0 {
		return h.items[0]
	}
	return nil
}

func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.items = []*ClipItem{}
}

func (h *History) Delete(index int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if index < 0 || index >= len(h.items) {
		return
	}
	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("正在删除历史记录中索引为%d的记录...", index)}
	h.items = append(h.items[:index], h.items[index+1:]...)
}

func (h *History) SetMaxSize(max uint) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.maxSize = max

	if max < (uint)(len(h.items)) {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("历史记录超过新设置的最大值%d，正在删除多余的记录...", max)}
		h.items = h.items[:max]
	}
}

type Group struct {
	Name         string
	Active       bool
	History      *History
	SingleDelete bool
}

func NewGroup(name string, active bool, maxSize uint) *Group {
	return &Group{
		Name:         name,
		Active:       active,
		History:      NewHistory(maxSize),
		SingleDelete: false,
	}
}

func syncLatestHistoryToGroup(history *History, group *Group) bool {
	if history == nil || group == nil || group.History == nil {
		return false
	}

	top := history.GetTop()
	if top == nil {
		return false
	}

	return group.History.Add(top.Clone())
}

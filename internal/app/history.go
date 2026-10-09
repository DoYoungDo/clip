package app

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

const hexDigits = "0123456789abcdef"

func appendHex16(dst []byte, v uint16) []byte {
	return append(dst,
		hexDigits[(v>>12)&0xf],
		hexDigits[(v>>8)&0xf],
		hexDigits[(v>>4)&0xf],
		hexDigits[v&0xf],
	)
}

func calcClipItemHash(itemType ItemType, content []byte) string {
	if itemType != TypeImage {
		return fmt.Sprintf("%x", md5.Sum(content))
	}

	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return fmt.Sprintf("%x", md5.Sum(content))
	}

	// 逐像素按 "%04x%04x%04x%04x" 的 ASCII 文本流写入哈希，
	// 输出与早期 fmt.Fprintf 实现保持完全一致，但避免每像素格式化的开销。
	bounds := img.Bounds()
	hasher := md5.New()
	_, _ = fmt.Fprintf(hasher, "%d:%d|", bounds.Dx(), bounds.Dy())
	buf := make([]byte, 0, 64*1024)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			buf = appendHex16(buf, uint16(r))
			buf = appendHex16(buf, uint16(g))
			buf = appendHex16(buf, uint16(b))
			buf = appendHex16(buf, uint16(a))
			if len(buf) >= 64*1024 {
				_, _ = hasher.Write(buf)
				buf = buf[:0]
			}
		}
	}
	if len(buf) > 0 {
		_, _ = hasher.Write(buf)
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func newClipItemWithHash(itemType ItemType, content []byte, hash string) *ClipItem {
	return &ClipItem{
		Type:    itemType,
		Content: append([]byte{}, content...),
		Hash:    hash,
		Time:    time.Now(),
		From:    FromLocal,
	}
}

func NewClipItem(itemType ItemType, content []byte) *ClipItem {
	return newClipItemWithHash(itemType, content, calcClipItemHash(itemType, content))
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
	return h.add(item, false)
}

// AddKeepLatest 添加项并移除历史中相同内容（类型 + Hash）的旧项，保证相同内容只保留最近一条。
func (h *History) AddKeepLatest(item *ClipItem) bool {
	return h.add(item, true)
}

func (h *History) add(item *ClipItem, keepLatest bool) bool {
	if item == nil {
		return false
	}

	h.mu.Lock()
	if !keepLatest && len(h.items) > 0 {
		top := h.items[0]
		if top != nil && top.Type == item.Type && top.Hash == item.Hash {
			h.mu.Unlock()
			return false
		}
	}

	// 允许重复，直接添加到最前面
	removed := 0
	if keepLatest {
		filtered := make([]*ClipItem, 0, len(h.items))
		for _, existing := range h.items {
			if existing == nil {
				continue
			}
			if existing.Type == item.Type && existing.Hash == item.Hash {
				removed++
				continue
			}
			filtered = append(filtered, existing)
		}
		h.items = filtered
	}

	h.items = append([]*ClipItem{item}, h.items...)
	if uint(len(h.items)) > h.maxSize {
		h.items = h.items[:h.maxSize]
	}
	h.mu.Unlock()

	if removed > 0 {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("已移除 %d 条相同内容的旧历史记录", removed)}
	}
	return true
}

// RemoveDuplicates 对当前历史做一次相同内容去重，每条内容只保留最新的那条。
func (h *History) RemoveDuplicates() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	seen := make(map[string]bool, len(h.items))
	filtered := make([]*ClipItem, 0, len(h.items))
	for _, item := range h.items {
		if item == nil {
			continue
		}
		key := fmt.Sprintf("%d:%s", item.Type, item.Hash)
		if seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, item)
	}

	removed := len(h.items) - len(filtered)
	h.items = filtered
	return removed
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

func (h *History) Delete(item *ClipItem) bool {
	if item == nil {
		return false
	}

	h.mu.Lock()
	index := -1
	for i, existing := range h.items {
		if existing == item {
			index = i
			break
		}
	}
	if index >= 0 {
		h.items = append(h.items[:index], h.items[index+1:]...)
	}
	h.mu.Unlock()

	if index < 0 {
		return false
	}

	global_log_channel <- LogEntry{Kind: KindInfo, Content: "正在删除历史记录..."}
	return true
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

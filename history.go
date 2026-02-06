package main

import (
	"crypto/md5"
	"fmt"
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
	Type     ItemType `json:"type"`
	Content  []byte `json:"content"`
	Hash     string `json:"hash"`
	Time     time.Time `json:"time"`
	From     ItemFrom `json:"from"`
}

func NewClipItem(itemType ItemType, content []byte) *ClipItem{
	return &ClipItem{
		Type:     itemType,
		Content:  append([]byte{}, content...),
		Hash:     fmt.Sprintf("%x", md5.Sum(content)),
		Time:     time.Now(),
		From:     FromLocal,
	}
}

func NewClipItemFromRemote(itemType ItemType, content []byte) *ClipItem{
	return &ClipItem{
		Type:     itemType,
		Content:  append([]byte{}, content...),
		Hash:     fmt.Sprintf("%x", md5.Sum(content)),
		Time:     time.Now(),
		From:     FromRemote,
	}
}

func (c *ClipItem) CloneToRemote() *ClipItem{
	return &ClipItem{
		Type:     c.Type,
		Content:  append([]byte{}, c.Content...),
		Hash:     c.Hash,
		Time:     c.Time,
		From:     FromRemote,
	}
}

func (c *ClipItem) Clone() *ClipItem{
	return &ClipItem{
		Type:     c.Type,
		Content:  append([]byte{}, c.Content...),
		Hash:     c.Hash,
		Time:     c.Time,
		From:     c.From,
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

	if len(h.items) > 0{
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
	h.items = append(h.items[:index], h.items[index+1:]...)
}

func (h *History) SetMaxSize(max uint) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.maxSize = max

	if max < (uint)(len(h.items)) {
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

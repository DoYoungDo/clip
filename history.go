package main

import (
	"sync"
	"time"
)

type ItemType int

const (
	TypeText ItemType = iota
	TypeImage
)

type ClipItem struct {
	Type     ItemType `json:"type"`
	Content  []byte `json:"content"`
	Hash     string `json:"hash"`
	Time     time.Time `json:"time"`
}

func (c *ClipItem) Clone() *ClipItem{
	return &ClipItem{
		Type:     c.Type,
		Content:  append([]byte{}, c.Content...),
		Hash:     c.Hash,
		Time:     c.Time,
	}
}

type History struct {
	items   []*ClipItem
	maxSize int
	mu      sync.RWMutex
}

func NewHistory(maxSize int) *History {
	return &History{
		items:   []*ClipItem{},
		maxSize: maxSize,
	}
}

func (h *History) Add(item *ClipItem) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.items) > 0{
		top := h.items[0]
		if top != nil && top.Type == item.Type && top.Hash == item.Hash {
			return
		}
	}

	// 允许重复，直接添加到最前面
	h.items = append([]*ClipItem{item}, h.items...)
	if len(h.items) > h.maxSize {
		h.items = h.items[:h.maxSize]
	}
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

type Group struct {
	Name         string
	Active       bool
	History      *History
	SingleDelete bool
}

func NewGroup(name string, active bool, maxSize int) *Group {
	return &Group{
		Name:         name,
		Active:       active,
		History:      NewHistory(maxSize),
		SingleDelete: false,
	}
}

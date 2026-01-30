package main

import (
	"sync"
	"time"
)

type ItemType int

const (
	TypeText ItemType = iota
	TypeImage
	TypeFile
)

type ClipItem struct {
	Type     ItemType
	Content  []byte
	Text     string
	FilePath string
	Time     time.Time
}

type Group struct {
	Name   string
	Items  []*ClipItem
	Active bool
}

type History struct {
	items   []*ClipItem
	groups  []*Group
	maxSize int
	mu      sync.RWMutex
}

func NewHistory(maxSize int) *History {
	return &History{
		items:   make([]*ClipItem, 0, maxSize),
		groups:  make([]*Group, 0),
		maxSize: maxSize,
	}
}

func (h *History) Add(item *ClipItem) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 允许重复，直接添加到最前面
	h.items = append([]*ClipItem{item}, h.items...)
	if len(h.items) > h.maxSize {
		h.items = h.items[:h.maxSize]
	}
	
	// 同步到激活的分组
	for _, group := range h.groups {
		if group.Active {
			group.Items = append([]*ClipItem{item}, group.Items...)
			if len(group.Items) > h.maxSize {
				group.Items = group.Items[:h.maxSize]
			}
		}
	}
}

func (h *History) GetAll() []*ClipItem {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	result := make([]*ClipItem, len(h.items))
	copy(result, h.items)
	return result
}

func (h *History) CreateGroup(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.groups = append(h.groups, &Group{
		Name:   name,
		Items:  make([]*ClipItem, 0),
		Active: false,
	})
}

func (h *History) DeleteGroup(index int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if index >= 0 && index < len(h.groups) {
		h.groups = append(h.groups[:index], h.groups[index+1:]...)
	}
}

func (h *History) RenameGroup(index int, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if index >= 0 && index < len(h.groups) {
		h.groups[index].Name = name
	}
}

func (h *History) ToggleGroupActive(index int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if index >= 0 && index < len(h.groups) {
		h.groups[index].Active = !h.groups[index].Active
	}
}

func (h *History) GetGroups() []*Group {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	result := make([]*Group, len(h.groups))
	copy(result, h.groups)
	return result
}

func (h *History) GetLatestText() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if len(h.items) > 0 {
		return h.items[0].Text
	}
	return "新分组"
}

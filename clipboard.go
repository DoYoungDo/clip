package main

import (
	"fmt"
	"sync"
	"time"

	"golang.design/x/clipboard"
)

type clipboardMonitor struct {
	reader chan *ClipItem
	writer chan *ClipItem
	done   chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
}

func (m *clipboardMonitor) Close() {
	m.once.Do(func() {
		close(m.done)
		m.wg.Wait()
	})
}

func (m *clipboardMonitor) Done() <-chan struct{} {
	return m.done
}

func selectClipboardChange(lastTextHash string, lastImageHash string, text []byte, image []byte) (*ClipItem, string, string) {
	textHash := ""
	imageHash := ""
	if len(text) > 0 {
		textHash = calcClipItemHash(TypeText, text)
	}
	if len(image) > 0 {
		imageHash = calcClipItemHash(TypeImage, image)
	}

	textChanged := textHash != "" && textHash != lastTextHash
	imageChanged := imageHash != "" && imageHash != lastImageHash

	switch {
	case imageChanged:
		return NewClipItem(TypeImage, image), textHash, imageHash
	case textChanged:
		return NewClipItem(TypeText, text), textHash, imageHash
	default:
		return nil, textHash, imageHash
	}
}

type echoSuppressor struct {
	mu        sync.Mutex
	itemType  ItemType
	hash      string
	expiresAt time.Time
}

func (e *echoSuppressor) Mark(item *ClipItem, ttl time.Duration) {
	if item == nil {
		return
	}

	e.mu.Lock()
	e.itemType = item.Type
	e.hash = item.Hash
	e.expiresAt = time.Now().Add(ttl)
	e.mu.Unlock()
}

func (e *echoSuppressor) ShouldSuppress(item *ClipItem) bool {
	if item == nil {
		return false
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if time.Now().After(e.expiresAt) {
		return false
	}
	if e.itemType == item.Type && e.hash != "" && e.hash == item.Hash {
		e.hash = ""
		e.expiresAt = time.Time{}
		return true
	}
	return false
}

func startMonitor() (*clipboardMonitor, error) {
	time.Sleep(time.Second)

	if err := clipboard.Init(); err != nil {
		global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("初始化剪贴板失败: %v", err)}
		return nil, err
	}

	monitor := &clipboardMonitor{
		reader: make(chan *ClipItem, 1),
		writer: make(chan *ClipItem, 1),
		done:   make(chan struct{}),
	}

	monitor.wg.Add(2)

	go func() {
		defer monitor.wg.Done()
		for {
			select {
			case <-monitor.done:
				return
			case item := <-monitor.writer:
				if item == nil {
					continue
				}
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("写入剪贴板: %s", formatMenuItem(item))}
				clipboard.Write(Ifel(item.Type == TypeImage, clipboard.FmtImage, clipboard.FmtText), item.Content)
			}
		}
	}()

	go func() {
		defer monitor.wg.Done()
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "开始监听剪贴板, 每200毫秒检查一次..."}
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		lastTextHash := ""
		lastImageHash := ""

		sendItem := func(item *ClipItem) bool {
			select {
			case <-monitor.done:
				return false
			case monitor.reader <- item:
				return true
			}
		}

		for {
			select {
			case <-monitor.done:
				return
			case <-ticker.C:
				text := clipboard.Read(clipboard.FmtText)
				image := clipboard.Read(clipboard.FmtImage)
				item, nextTextHash, nextImageHash := selectClipboardChange(lastTextHash, lastImageHash, text, image)
				lastTextHash = nextTextHash
				lastImageHash = nextImageHash
				if item != nil && !sendItem(item) {
					return
				}
			}
		}
	}()

	return monitor, nil
}

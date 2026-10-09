package app

import (
	"bytes"
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

	pendingMu    sync.Mutex
	pendingWrite *ClipItem
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

func (m *clipboardMonitor) setPendingWrite(item *ClipItem) {
	m.pendingMu.Lock()
	m.pendingWrite = item
	m.pendingMu.Unlock()
}

func (m *clipboardMonitor) pendingWriteItem() *ClipItem {
	m.pendingMu.Lock()
	defer m.pendingMu.Unlock()
	return m.pendingWrite
}

type monitorCache struct {
	text      []byte
	image     []byte
	textHash  string
	imageHash string
}

// poll 读取当前剪贴板快照并返回变化项；内容与上轮完全相同时直接跳过，避免重复计算昂贵的图片哈希。
func (c *monitorCache) poll(text []byte, image []byte, pending *ClipItem) *ClipItem {
	if bytes.Equal(text, c.text) && bytes.Equal(image, c.image) {
		return nil
	}

	item, textHash, imageHash := selectClipboardChange(c.textHash, c.imageHash, text, image, pending)
	c.text = append(c.text[:0], text...)
	c.image = append(c.image[:0], image...)
	c.textHash = textHash
	c.imageHash = imageHash
	return item
}

func selectClipboardChange(lastTextHash string, lastImageHash string, text []byte, image []byte, pending *ClipItem) (*ClipItem, string, string) {
	textHash := ""
	imageHash := ""
	if len(text) > 0 {
		textHash = clipItemHash(TypeText, text, pending)
	}
	if len(image) > 0 {
		imageHash = clipItemHash(TypeImage, image, pending)
	}

	textChanged := textHash != "" && textHash != lastTextHash
	imageChanged := imageHash != "" && imageHash != lastImageHash

	switch {
	case imageChanged:
		return newClipItemWithHash(TypeImage, image, imageHash), textHash, imageHash
	case textChanged:
		return newClipItemWithHash(TypeText, text, textHash), textHash, imageHash
	default:
		return nil, textHash, imageHash
	}
}

// clipItemHash 优先复用写回剪贴板时已知的哈希，避免对同一内容重复解码计算。
func clipItemHash(itemType ItemType, content []byte, pending *ClipItem) string {
	if pending != nil && pending.Type == itemType && bytes.Equal(pending.Content, content) {
		return pending.Hash
	}
	return calcClipItemHash(itemType, content)
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
				monitor.setPendingWrite(item)
				clipboard.Write(Ifel(item.Type == TypeImage, clipboard.FmtImage, clipboard.FmtText), item.Content)
			}
		}
	}()

	go func() {
		defer monitor.wg.Done()
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "开始监听剪贴板, 每200毫秒检查一次..."}
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		cache := &monitorCache{}

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
				item := cache.poll(text, image, monitor.pendingWriteItem())
				if item != nil && !sendItem(item) {
					return
				}
			}
		}
	}()

	return monitor, nil
}

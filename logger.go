package main

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"
)

type LogKind string

const (
	KindInfo  LogKind = "INFO"
	KindError LogKind = "ERR"
)

type LogEntry struct {
	Kind    LogKind
	Content string
}

type AppLogger struct {
	entries chan LogEntry
	mu      sync.Mutex
	buffer  bytes.Buffer
}

var global_log_channel = make(chan LogEntry, 128)

func NewAppLogger() *AppLogger {
	logger := &AppLogger{entries: make(chan LogEntry, 128)}

	go func() {
		for entry := range logger.entries {
			logger.mu.Lock()
			fmt.Fprintf(&logger.buffer, "%v [%v] %v", time.Now().Format("2006-01-02 15:04:05"), entry.Kind, fmt.Sprintln(entry.Content))
			logger.mu.Unlock()
		}
	}()

	return logger
}

func (l *AppLogger) FlushToFile(enabled bool) {
	if !enabled {
		return
	}

	for i := 0; i < 10 && len(l.entries) > 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}

	path := getLogPath()
	if err := ensureParentDir(path); err != nil {
		fmt.Fprintf(os.Stderr, "创建日志目录失败: %v\n", err)
		return
	}

	l.mu.Lock()
	data := append([]byte(nil), l.buffer.Bytes()...)
	l.mu.Unlock()

	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "写入日志失败: %v\n", err)
	}
}

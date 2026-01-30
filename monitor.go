package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"strings"
	"time"

	"golang.design/x/clipboard"
)

func startMonitor(history *History, onUpdate func()) {
	time.Sleep(time.Second)
	
	if err := clipboard.Init(); err != nil {
		return
	}

	var lastText []byte
	var lastImage []byte
	var lastFilePath string
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// 先检查是否有文件
		filePath := getFilePath()
		if filePath != "" {
			if filePath != lastFilePath {
				history.Add(&ClipItem{
					Type:     TypeFile,
					Content:  []byte(filePath),
					Text:     filePath,
					FilePath: filePath,
					Time:     time.Now(),
				})
				lastFilePath = filePath
				lastText = nil
				if onUpdate != nil {
					onUpdate()
				}
			}
			continue
		} else {
			// 没有文件时清空文件缓存
			lastFilePath = ""
		}

		// 监听文本
		text := clipboard.Read(clipboard.FmtText)
		if len(text) > 0 && !bytes.Equal(text, lastText) {
			textStr := string(text)
			itemType := TypeText
			displayText := textStr
			filePath := ""
			
			// 检查是否是 file:// 格式（Linux）
			if strings.HasPrefix(textStr, "file://") {
				itemType = TypeFile
				filePath = strings.TrimPrefix(textStr, "file://")
				filePath = strings.TrimSpace(filePath)
				displayText = filePath
			}
			
			history.Add(&ClipItem{
				Type:     itemType,
				Content:  append([]byte(nil), text...),
				Text:     displayText,
				FilePath: filePath,
				Time:     time.Now(),
			})
			
			lastText = append([]byte(nil), text...)
			if onUpdate != nil {
				onUpdate()
			}
		}

		// 监听图片
		image := clipboard.Read(clipboard.FmtImage)
		if len(image) > 0 && !bytes.Equal(image, lastImage) {
			hash := fmt.Sprintf("%x", md5.Sum(image))
			history.Add(&ClipItem{
				Type:     TypeImage,
				Content:  append([]byte(nil), image...),
				Text:     fmt.Sprintf("图片 [%s]", hash[:8]), // 显示前8位MD5
				FilePath: hash, // 完整 MD5 用于去重
				Time:     time.Now(),
			})
			lastImage = append([]byte(nil), image...)
			if onUpdate != nil {
				onUpdate()
			}
		}
	}
}

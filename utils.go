package main

import (
	"os"
	"path/filepath"
)

func Ifel[T any](ok bool, a T, b T) T {
	if ok {
		return a
	}
	return b
}

func BoolPtr(b bool) *bool {
	return &b
}

func getConfigPath() string {
    // 获取可执行文件路径
    execPath, err := os.Executable()
    if err != nil {
        return "config.json" // 降级到当前目录
    }
    
    // 获取应用目录
    appDir := filepath.Dir(execPath)
    return filepath.Join(appDir, "config.json")
}


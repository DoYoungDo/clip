package app

import (
	"os"
	"path/filepath"
)

// singleInstanceLockPath 返回单实例锁文件路径（与配置同目录）。
func singleInstanceLockPath() string {
	return filepath.Join(getAppDataDir(), "clip.lock")
}

// acquireSingleInstance 尝试获取单实例锁；已有实例在运行时返回 ok=false。
func acquireSingleInstance() (*os.File, bool) {
	return acquireInstanceLockAt(singleInstanceLockPath())
}

// acquireInstanceLockAt 在指定路径获取单实例锁，必要时创建父目录；
// 锁机制自身失败（如锁文件无法创建）时放行启动，避免误阻止程序运行。
func acquireInstanceLockAt(path string) (*os.File, bool) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, true
	}
	return lockSingleInstanceFile(path)
}

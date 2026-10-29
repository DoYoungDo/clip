//go:build darwin || linux

package app

import (
	"errors"
	"os"
	"syscall"
)

// lockSingleInstanceFile 对锁文件加非阻塞排它锁；锁被占用时返回 ok=false。
// 持有返回的文件句柄即持有锁，句柄关闭或进程退出时自动释放。
func lockSingleInstanceFile(path string) (*os.File, bool) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, true
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, false
		}
		return nil, true
	}
	return file, true
}

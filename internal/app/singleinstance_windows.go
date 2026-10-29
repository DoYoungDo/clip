//go:build windows

package app

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockSingleInstanceFile 对锁文件加非阻塞排它锁（LockFileEx）；锁被占用时返回 ok=false。
// 持有返回的文件句柄即持有锁，句柄关闭或进程退出时自动释放。
func lockSingleInstanceFile(path string) (*os.File, bool) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, true
	}
	overlapped := new(windows.Overlapped)
	err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlapped)
	if err != nil {
		file.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, false
		}
		return nil, true
	}
	return file, true
}

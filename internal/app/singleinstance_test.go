package app

import (
	"path/filepath"
	"testing"
)

func TestLockSingleInstanceFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.lock")

	first, ok := lockSingleInstanceFile(path)
	if !ok {
		t.Fatalf("首次获取锁应成功")
	}
	if first == nil {
		t.Fatalf("首次获取锁应返回锁文件句柄")
	}

	if second, ok := lockSingleInstanceFile(path); ok {
		if second != nil {
			second.Close()
		}
		t.Fatalf("锁被占用时再次获取应失败")
	}

	first.Close()

	third, ok := lockSingleInstanceFile(path)
	if !ok {
		t.Fatalf("释放锁后应能重新获取")
	}
	if third != nil {
		third.Close()
	}
}

func TestAcquireInstanceLockCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "clip.lock")

	first, ok := acquireInstanceLockAt(path)
	if !ok || first == nil {
		t.Fatalf("父目录不存在时应自动创建并获取锁: ok=%v", ok)
	}
	defer first.Close()

	if second, ok := acquireInstanceLockAt(path); ok {
		if second != nil {
			second.Close()
		}
		t.Fatalf("锁被占用时再次获取应失败")
	}
}

func TestSingleInstanceLockPath(t *testing.T) {
	path := singleInstanceLockPath()
	if filepath.Base(path) != "clip.lock" {
		t.Fatalf("锁文件名错误: %s", path)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("锁文件路径应为绝对路径: %s", path)
	}
}

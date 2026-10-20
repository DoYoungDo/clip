package app

import (
	"sync/atomic"
	"time"
)

// stateSaveDebounce 合并短时间内的多次变更，避免频繁写盘。
var stateSaveDebounce = 500 * time.Millisecond

// atomicString 提供并发安全的字符串读写，供后台保存协程读取配置。
type atomicString struct {
	value atomic.Value
}

func (s *atomicString) Load() string {
	v, _ := s.value.Load().(string)
	return v
}

func (s *atomicString) Store(v string) {
	s.value.Store(v)
}

// requestSave 非阻塞地请求一次落盘，菜单事件回调调用它不会因写盘而卡顿。
func (a *App) requestSave() {
	if a.saveTrigger == nil {
		return
	}
	select {
	case a.saveTrigger <- struct{}{}:
	default:
	}
}

// startStateSaver 启动后台保存协程，与 stopStateSaver 成对使用。
func (a *App) startStateSaver() {
	if a.saveStop != nil {
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	a.saveStop = stop
	a.saveDone = done
	go a.stateSaverLoop(stop, done)
}

// stopStateSaver 停止后台保存协程；未落盘的变更由退出前的同步保存兜底。
func (a *App) stopStateSaver() {
	if a.saveStop == nil {
		return
	}
	close(a.saveStop)
	<-a.saveDone
	a.saveStop = nil
}

func (a *App) stateSaverLoop(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	timer := time.NewTimer(stateSaveDebounce)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for {
		select {
		case <-stop:
			return
		case <-a.saveTrigger:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(stateSaveDebounce)
		case <-timer.C:
			a.saveLocalStateSilent()
		}
	}
}

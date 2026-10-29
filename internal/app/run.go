package app

import (
	"github.com/energye/systray"
)

// Run 启动托盘应用：加载状态、启动剪贴板监听并进入事件循环。
// 启动时获取单实例锁，已有实例在运行时直接退出，避免多来源重复启动出现多个托盘图标。
func Run() {
	lock, ok := acquireSingleInstance()
	if !ok {
		return
	}
	if lock != nil {
		defer lock.Close()
	}

	logger := NewAppLogger()
	global_log_channel = logger.entries
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "程序启动"}

	application := NewApp(logger)
	application.loadLocalState()

	monitor, err := startMonitor()
	if err != nil {
		application.saveLocalState()
		logger.FlushToFile(application.configSaveLogToLocal.Load())
		return
	}
	application.monitor = monitor
	application.writer = monitor.writer
	application.startStateSaver()

	go application.consumeClipboardItems()

	systray.Run(application.onTrayReady, application.shutdown)
}

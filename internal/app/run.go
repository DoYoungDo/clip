package app

import (
	"github.com/energye/systray"
)

// Run 启动托盘应用：加载状态、启动剪贴板监听并进入事件循环。
func Run() {
	logger := NewAppLogger()
	global_log_channel = logger.entries
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "程序启动"}

	application := NewApp(logger)
	application.loadLocalState()

	monitor, err := startMonitor()
	if err != nil {
		application.saveLocalState()
		logger.FlushToFile(application.configSaveLogToLocal)
		return
	}
	application.monitor = monitor
	application.writer = monitor.writer

	go application.consumeClipboardItems()

	systray.Run(application.onTrayReady, application.shutdown)
}

package main

import (
	"github.com/energye/systray"
)

func main() {
	logger := NewAppLogger()
	global_log_channel = logger.entries
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "程序启动"}

	app := NewApp(logger)
	app.loadLocalState()

	monitor, err := startMonitor()
	if err != nil {
		app.saveLocalState()
		logger.FlushToFile(config_save_log_to_local)
		return
	}
	app.monitor = monitor
	app.writer = monitor.writer

	go app.consumeClipboardItems()

	systray.Run(app.onTrayReady, app.shutdown)
}

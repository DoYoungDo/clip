// Package webui 提供网页管理的嵌入式静态页面资源。
package webui

import (
	"embed"
	"io/fs"
)

//go:embed static
var embedded embed.FS

// FS 返回管理页静态资源文件系统（根目录为 static 目录内容）。
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

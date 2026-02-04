package main

import (
	"os"
	"path/filepath"
	"regexp"
)

func Ifel[T any](ok bool, a T, b T) T {
	if ok {
		return a
	}
	return b
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

func getColor(str string) (r string, g string, b string, base int, ok bool) {
	pattern := `(?i)^(?:#(?:([\da-f][\da-f])([\da-f][\da-f])([\da-f][\da-f])|([\da-f])([\da-f])([\da-f]))|\(?(\d|1?\d{2}|2[0-5]{2})\s*,\s*(\d|1?\d{2}|2[0-5]{2})\s*,\s*(\d|1?\d{2}|2[0-5]{2})(?:\s*,\s*(?:\d|1?\d{2}|2[0-5]{2}))?\)?)$`
	reg, err := regexp.Compile(pattern)
	if err != nil{
		return "", "", "", 0, false
	}

	groups := reg.FindStringSubmatch(str)
	if len(groups) < 9 {
		return "", "", "", 0, false
	}

	r = Ifel(groups[1] != "", groups[1], Ifel(groups[4] != "", groups[4], Ifel(groups[7] != "", groups[7], "")))
	g = Ifel(groups[2] != "", groups[2], Ifel(groups[5] != "", groups[5], Ifel(groups[8] != "", groups[8], "")))
	b = Ifel(groups[3] != "", groups[3], Ifel(groups[6] != "", groups[6], Ifel(groups[9] != "", groups[9], "")))
	ok = r != "" && g != "" && b != ""

	return r, g, b, Ifel(groups[1] != "", 16, 10), ok
}

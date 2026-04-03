package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
)

const appDataDirName = "Clip"

var legacyConfigPathOverride string

func getAppDataDir() string {
	configDir, err := os.UserConfigDir()
	if err == nil {
		return filepath.Join(configDir, appDataDirName)
	}

	execPath, err := os.Executable()
	if err == nil {
		return filepath.Dir(execPath)
	}

	return "."
}

func ensureParentDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
}

func Ifel[T any](ok bool, a T, b T) T {
	if ok {
		return a
	}
	return b
}
func IfelFunc[T any](ok bool, a func() T, b func() T) T {
	if ok {
		return a()
	}
	return b()
}

func getConfigPath() string {
	return filepath.Join(getAppDataDir(), "config.json")
}

func getLegacyConfigPath() string {
	if legacyConfigPathOverride != "" {
		return legacyConfigPathOverride
	}
	execPath, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(execPath), "config.json")
}

func getLogPath() string {
	return filepath.Join(getAppDataDir(), "clip.log")
}

func getColor(str string) (r string, g string, b string, base int, ok bool) {
	pattern := `(?i)^(?:#(?:([\da-f][\da-f])([\da-f][\da-f])([\da-f][\da-f])|([\da-f])([\da-f])([\da-f]))|\(?(\d|1?\d{2}|2[0-5]{2})\s*,\s*(\d|1?\d{2}|2[0-5]{2})\s*,\s*(\d|1?\d{2}|2[0-5]{2})(?:\s*,\s*(?:\d|1?\d{2}|2[0-5]{2}))?\)?)$`
	reg, err := regexp.Compile(pattern)
	if err != nil {
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
func openDir(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default: // linux
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

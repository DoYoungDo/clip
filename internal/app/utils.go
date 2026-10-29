package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

const appDataDirName = "Clip"

var legacyConfigPathOverride string

var configPathOverride string

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

func getConfigPath() string {
	if configPathOverride != "" {
		return configPathOverride
	}
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

var colorPattern = regexp.MustCompile(`(?i)^(?:#(?:([\da-f][\da-f])([\da-f][\da-f])([\da-f][\da-f])|([\da-f])([\da-f])([\da-f]))|\(?(\d|1?\d{2}|2[0-5]{2})\s*,\s*(\d|1?\d{2}|2[0-5]{2})\s*,\s*(\d|1?\d{2}|2[0-5]{2})(?:\s*,\s*(?:\d|1?\d{2}|2[0-5]{2}))?\)?)$`)

func getColor(str string) (r string, g string, b string, base int, ok bool) {
	groups := colorPattern.FindStringSubmatch(str)
	if len(groups) < 9 {
		return "", "", "", 0, false
	}

	r = Ifel(groups[1] != "", groups[1], Ifel(groups[4] != "", groups[4], Ifel(groups[7] != "", groups[7], "")))
	g = Ifel(groups[2] != "", groups[2], Ifel(groups[5] != "", groups[5], Ifel(groups[8] != "", groups[8], "")))
	b = Ifel(groups[3] != "", groups[3], Ifel(groups[6] != "", groups[6], Ifel(groups[9] != "", groups[9], "")))
	ok = r != "" && g != "" && b != ""

	return r, g, b, Ifel(groups[1] != "", 16, 10), ok
}

var (
	baseBinPattern = regexp.MustCompile(`(?i)^0b[01]+$`)
	baseOctPattern = regexp.MustCompile(`(?i)^0o[0-7]+$`)
	baseHexPattern = regexp.MustCompile(`(?i)^0x[\da-f]+$`)
	baseDecPattern = regexp.MustCompile(`^\d+$`)
)

// parseBaseNumber 解析整数文本及其进制：0b/0o/0x 前缀分别识别二/八/十六进制，无前缀纯数字按十进制。
func parseBaseNumber(str string) (value uint64, base int, ok bool) {
	var digits string
	switch {
	case baseBinPattern.MatchString(str):
		base, digits = 2, str[2:]
	case baseOctPattern.MatchString(str):
		base, digits = 8, str[2:]
	case baseHexPattern.MatchString(str):
		base, digits = 16, str[2:]
	case baseDecPattern.MatchString(str):
		base, digits = 10, str
	default:
		return 0, 0, false
	}

	v, err := strconv.ParseUint(digits, base, 64)
	if err != nil {
		return 0, 0, false
	}
	return v, base, true
}

// formatBaseValue 按目标进制格式化数值：二进制 0b、八进制 0o、十六进制 0x 前缀，十进制无前缀。
func formatBaseValue(value uint64, base int) string {
	switch base {
	case 2:
		return "0b" + strconv.FormatUint(value, 2)
	case 8:
		return "0o" + strconv.FormatUint(value, 8)
	case 16:
		return "0x" + strings.ToUpper(strconv.FormatUint(value, 16))
	default:
		return strconv.FormatUint(value, 10)
	}
}
func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
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
	return startDetached(cmd)
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // linux
		cmd = exec.Command("xdg-open", url)
	}
	return startDetached(cmd)
}

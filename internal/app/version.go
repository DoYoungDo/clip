package app

const (
	appName      = "Clip"
	appLicense   = "GPL-3.0"
	appAuthor    = "DoYoung"
	appAuthorURL = "https://github.com/DoYoungDo"
	appRepoURL   = "https://github.com/DoYoungDo/clip"
)

// version 由构建脚本通过 -ldflags "-X clip/internal/app.version=$(git describe --tags --always)" 注入。
var version = "dev"

func currentVersion() string {
	if version == "" {
		return "dev"
	}
	return version
}

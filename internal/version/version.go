package version

// Version is the application version, injected at build time via ldflags.
// Example: go build -ldflags "-X github.com/dujiao-next/internal/version.Version=v1.2.3"
var Version = "v1.0.0"

// BuildType 标识二进制来源。CI 正式发布（goreleaser / Dockerfile）注入 "release"，
// 本地 go build / go run 保持默认 "source"。
// 一键升级只对 release 产物开放 —— 本地构建的版本号是占位值，
// 自动替换会把开发者自己编译的二进制覆盖掉。
var BuildType = BuildTypeSource

const (
	BuildTypeSource  = "source"
	BuildTypeRelease = "release"
)

// IsReleaseBuild 判断当前二进制是否为 CI 正式发布产物。
func IsReleaseBuild() bool { return BuildType == BuildTypeRelease }

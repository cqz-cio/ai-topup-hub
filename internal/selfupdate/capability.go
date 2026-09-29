// Package selfupdate 实现后台一键升级：从 GitHub Release 拉取当前平台的二进制归档，
// 校验 sha256 后原子替换正在运行的可执行文件，并在具备进程守护时自动重启。
//
// 明确不支持容器部署。容器内替换 /app/dujiao-next 只对当前容器生命周期有效，
// 一旦 docker restart / compose up / 宿主重启，进程又会回到镜像层里的旧二进制，
// 表现为「升级成功后过几天自己变回旧版」。容器的正确升级路径是拉新镜像重建容器，
// 因此这里探测到容器环境时直接阻断，由前端改为展示 compose 升级命令。
package selfupdate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/version"
)

// Deployment 当前进程的部署形态
type Deployment string

const (
	// DeploymentBinary 裸二进制部署（systemd / 手工启动）
	DeploymentBinary Deployment = "binary"
	// DeploymentContainer 容器部署（Docker / Podman / Kubernetes）
	DeploymentContainer Deployment = "container"
)

// BlockReason 一键升级不可用的原因码。前端据此渲染对应的手动升级指引，
// 不直接透传后端文案，保证 i18n 由前端统一处理。
type BlockReason string

const (
	BlockNone           BlockReason = ""
	BlockContainer      BlockReason = "container"
	BlockUnsupportedOS  BlockReason = "unsupported_os"
	BlockSourceBuild    BlockReason = "source_build"
	BlockDirNotWritable BlockReason = "dir_not_writable"
	BlockExecNotFound   BlockReason = "exec_not_found"
)

// Supervisor 进程守护方式，决定替换二进制后能否自动重启
type Supervisor string

const (
	// SupervisorSystemd 由 systemd 托管，进程退出后靠 Restart= 策略拉起
	SupervisorSystemd Supervisor = "systemd"
	// SupervisorNone 无守护进程，退出即停服，只能提示用户手动重启
	SupervisorNone Supervisor = "none"
)

// Capability 一键升级能力探测结果，直接序列化给后台前端
type Capability struct {
	// CanUpdate 是否允许下载并替换二进制
	CanUpdate bool `json:"can_update"`
	// CanRestart 替换完成后能否由后端自动重启进程；false 时需用户手动重启
	CanRestart bool `json:"can_restart"`
	// BlockReason CanUpdate 为 false 时的原因码
	BlockReason BlockReason `json:"block_reason,omitempty"`
	Deployment  Deployment  `json:"deployment"`
	Supervisor  Supervisor  `json:"supervisor"`
	BuildType   string      `json:"build_type"`
	// RestartPolicy systemd unit 的 Restart= 取值（查不到时为空），
	// 决定 CanRestart —— 排障时能直接看出是策略不支持还是没探测到 unit
	RestartPolicy string `json:"restart_policy,omitempty"`
	// ExecPath 当前可执行文件的真实路径（已解析软链），便于用户核对升级目标
	ExecPath  string `json:"exec_path,omitempty"`
	Platform  string `json:"platform"`
	AssetName string `json:"asset_name,omitempty"`
	// HasBackup 是否存在可回滚的上一版本备份
	HasBackup bool `json:"has_backup"`
	// Backup 备份来源、迁移与启动状态。无备份、或元数据缺失/损坏时为 nil。
	Backup *BackupInfo `json:"backup,omitempty"`
	// RollbackUnsafe 回滚是否需要用户确认风险后强制执行。
	// 迁移已开始、完整启动过、或元数据不可信而无法判断时都为 true —— 后者同样危险，
	// 从旧版本升上来的第一次就属于这种情况。前端据此决定是否弹风险确认并带 force。
	RollbackUnsafe bool `json:"rollback_unsafe"`
}

// Detect 探测当前运行环境是否支持一键升级。
// 判定顺序按「越根本越先」排列，第一个不满足的条件即为阻断原因。
func Detect() Capability {
	c := Capability{
		Deployment: DeploymentBinary,
		Supervisor: SupervisorNone,
		BuildType:  version.BuildType,
		Platform:   runtime.GOOS + "/" + runtime.GOARCH,
	}

	if InContainer() {
		c.Deployment = DeploymentContainer
		c.BlockReason = BlockContainer
		return c
	}

	// Windows 上正在运行的 exe 虽可重命名，但没有等价的守护/重启约定，
	// 且不是本项目的目标部署环境，直接阻断而不是留一条没验证过的路径。
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		c.BlockReason = BlockUnsupportedOS
		return c
	}

	// 本地 go build 的版本号是占位值 v1.0.0，任何线上发行版都会被判定为「有新版本」，
	// 放开升级等于把开发者自己编译的二进制悄悄换掉。
	if !version.IsReleaseBuild() {
		c.BlockReason = BlockSourceBuild
		return c
	}

	execPath, err := ExecutablePath()
	if err != nil {
		c.BlockReason = BlockExecNotFound
		return c
	}
	c.ExecPath = execPath

	// 替换走 rename，需要的是二进制所在目录的写权限，而不是文件本身的写权限。
	if !dirWritable(filepath.Dir(execPath)) {
		c.BlockReason = BlockDirNotWritable
		return c
	}

	c.CanUpdate = true
	c.AssetName = PlatformAssetSuffix()
	c.Supervisor = detectSupervisor()
	// 只确认「由 systemd 启动」是不够的：unit 的 Restart= 策略才决定进程退出后会不会被拉起。
	// Restart=no 的 unit 一样会注入 INVOCATION_ID，但点重启就是永久停服。
	//
	// 查不到配置时一律判定为不能重启：查询失败证明不了任何事，而两种猜错的代价不对称——
	// 猜「能重启」猜错是服务直接下线，猜「不能重启」猜错只是让用户多敲一条 systemctl restart。
	if c.Supervisor == SupervisorSystemd {
		if cfg, ok := systemdRestartConfig(); ok {
			c.RestartPolicy = cfg.Restart
			c.CanRestart = restartConfigRelaunches(cfg)
		}
	}
	if _, err := os.Stat(backupPath(execPath)); err == nil {
		c.HasBackup = true
		info, known, err := ReadBackupInfo(execPath)
		if err == nil && known {
			c.Backup = &info
			c.RollbackUnsafe = rollbackUnsafe(info, true)
		} else {
			// 元数据缺失或损坏：无法证明新版本还没跑过，按不安全处理，
			// 让前端弹风险确认并带 force。宁可多问一次，也不能默默放行。
			c.RollbackUnsafe = true
		}
	}
	return c
}

// ExecutablePath 返回当前可执行文件解析软链后的绝对路径。
// 必须解析软链：常见部署会用 /usr/local/bin/dujiao-next -> /opt/dujiao-next/dujiao-next，
// 直接替换软链会把链接本身变成普通文件。
func ExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}
	return resolved, nil
}

// InContainer 判断进程是否运行在容器内。
// 覆盖 Docker、Podman、Kubernetes 三种常见形态；任一命中即认为是容器。
func InContainer() bool {
	// Docker 官方镜像运行时一定存在
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Podman / CRI-O
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	// cgroup v1 下 PID 1 的 cgroup 路径带容器运行时标记；
	// cgroup v2 纯净环境下可能只有 "0::/"，此时靠上面几项兜底。
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		for _, marker := range []string{"/docker/", "/docker-", "containerd", "kubepods", "/lxc/", "libpod"} {
			if strings.Contains(content, marker) {
				return true
			}
		}
	}
	return false
}

// detectSupervisor 判断进程是否被 systemd 托管。
// INVOCATION_ID 由 systemd 235+ 为每个服务单元注入，JOURNAL_STREAM 在日志接管时注入，
// 二者都是用户态可读的可靠判据，比解析 /proc/1/comm 更准（容器里 PID 1 也可能是 systemd）。
func detectSupervisor() Supervisor {
	if os.Getenv("INVOCATION_ID") != "" || os.Getenv("JOURNAL_STREAM") != "" {
		return SupervisorSystemd
	}
	return SupervisorNone
}

// systemctlTimeout 查询 unit 属性的超时。systemctl show 走本地 D-Bus，正常在毫秒级；
// 给 2 秒是为了在 D-Bus 异常时不把能力探测接口拖住。
const systemctlTimeout = 2 * time.Second

// systemdUnit 从 cgroup 路径解析当前进程所属的 systemd service 名。
//
//	cgroup v2: "0::/system.slice/dujiao-next.service"
//	cgroup v1: "1:name=systemd:/system.slice/dujiao-next.service"
//
// 用 cgroup 而不是猜固定名字：unit 名由用户自己定，文档里叫 dujiao-next.service，
// 实际部署可能是任意名称。
func systemdUnit() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 {
			continue
		}
		for _, seg := range strings.Split(fields[2], "/") {
			if strings.HasSuffix(seg, ".service") {
				return seg
			}
		}
	}
	return ""
}

// unitRestartConfig 决定「以 RestartExitCode 退出后会不会被拉起」所需的全部 unit 配置。
type unitRestartConfig struct {
	// Restart unit 的 Restart= 取值
	Restart string
	// SuccessExitStatus 声明为「成功」的额外退出码集合
	SuccessExitStatus map[int]bool
	// RestartPreventExitStatus 明确禁止触发重启的退出码集合
	RestartPreventExitStatus map[int]bool
}

// systemdRestartConfig 查询当前 unit 与重启判定相关的三项配置。
// 查询失败时返回 (零值, false) —— 调用方必须按「无法证明能重启」处理。
func systemdRestartConfig() (unitRestartConfig, bool) {
	unit := systemdUnit()
	if unit == "" {
		return unitRestartConfig{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer cancel()

	// 一次取三项：只看 Restart= 不足以判断退出码 70 会不会被拉起——
	// SuccessExitStatus=70 会把它变成「成功退出」从而绕过 on-failure，
	// RestartPreventExitStatus=70 则会直接压过 always/on-failure 阻止重启。
	// --value 需要 systemd 230+；INVOCATION_ID 本身就要 232+，走到这里必然满足。
	out, err := exec.CommandContext(ctx, "systemctl", "show",
		"--property=Restart",
		"--property=SuccessExitStatus",
		"--property=RestartPreventExitStatus",
		unit).Output()
	if err != nil {
		return unitRestartConfig{}, false
	}

	cfg := unitRestartConfig{
		SuccessExitStatus:        map[int]bool{},
		RestartPreventExitStatus: map[int]bool{},
	}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "Restart":
			cfg.Restart = strings.TrimSpace(value)
		case "SuccessExitStatus":
			cfg.SuccessExitStatus = parseExitStatusSet(value)
		case "RestartPreventExitStatus":
			cfg.RestartPreventExitStatus = parseExitStatusSet(value)
		}
	}
	if cfg.Restart == "" {
		// 属性拿不到值说明查询结果不可信，不能当成「systemd 返回了默认值」
		return unitRestartConfig{}, false
	}
	return cfg, true
}

// parseExitStatusSet 解析 systemd 的退出码列表。取值形如 "70 75" 或 "1 SIGTERM"，
// 信号名对本判定无意义（我们只关心退出码），忽略即可。
func parseExitStatusSet(value string) map[int]bool {
	set := map[int]bool{}
	for _, field := range strings.Fields(value) {
		if n, err := strconv.Atoi(field); err == nil {
			set[n] = true
		}
	}
	return set
}

// restartConfigRelaunches 报告以 RestartExitCode 退出后 systemd 会不会重新拉起本服务。
//
// 判定顺序与 systemd 一致：
//  1. RestartPreventExitStatus 命中则一票否决，压过任何 Restart= 策略
//  2. SuccessExitStatus 决定这次退出算「成功」还是「失败」
//  3. 再按 Restart= 策略判断该类退出会不会触发重启
//
// on-abnormal / on-abort / on-watchdog 只认信号、超时与 watchdog，从不看退出码；
// Restart= 的默认值是 no，也就是什么都不做。
func restartConfigRelaunches(cfg unitRestartConfig) bool {
	if cfg.RestartPreventExitStatus[RestartExitCode] {
		return false
	}
	exitIsSuccess := cfg.SuccessExitStatus[RestartExitCode]

	switch cfg.Restart {
	case "always":
		return true
	case "on-failure":
		return !exitIsSuccess
	case "on-success":
		return exitIsSuccess
	default:
		return false
	}
}

// dirWritable 通过实际创建临时文件判断目录可写。
// 不用 unix.Access：进程可能以非 root 运行且属主/属组与 stat 位的组合难以准确推断，
// 直接试一次最可靠。
func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".dujiao-update-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// PlatformAssetSuffix 返回当前平台对应的 goreleaser 归档文件名后缀。
// 与 .goreleaser.yaml 的 name_template 保持一致：
//
//	{{ProjectName}}_{{.Tag}}_{{title .Os}}_{{x86_64|i386|其它原样}}
//
// 例：dujiao-next_v1.5.0_Linux_x86_64.tar.gz
func PlatformAssetSuffix() string {
	osPart := strings.ToUpper(runtime.GOOS[:1]) + runtime.GOOS[1:]

	archPart := runtime.GOARCH
	switch runtime.GOARCH {
	case "amd64":
		archPart = "x86_64"
	case "386":
		archPart = "i386"
	}

	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("_%s_%s%s", osPart, archPart, ext)
}

// backupPath 上一版本二进制的备份路径
func backupPath(execPath string) string {
	return execPath + ".backup"
}

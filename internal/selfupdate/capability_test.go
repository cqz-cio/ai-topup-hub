package selfupdate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dujiao-next/internal/version"
)

// TestDetectBlocksSourceBuild 本地构建（BuildType=source）必须被阻断，
// 否则 go run 起来的开发进程会把自己的二进制换成线上发行版。
func TestDetectBlocksSourceBuild(t *testing.T) {
	restore := version.BuildType
	version.BuildType = version.BuildTypeSource
	t.Cleanup(func() { version.BuildType = restore })

	c := Detect()
	if c.CanUpdate {
		t.Error("source build should not be allowed to self-update")
	}
	if c.BlockReason == BlockNone {
		t.Error("blocked capability must carry a reason code")
	}
	// 容器内跑测试时会先命中容器判定，两种原因码都算正确
	if c.BlockReason != BlockSourceBuild && c.BlockReason != BlockContainer {
		t.Errorf("BlockReason = %q, want source_build (or container in CI)", c.BlockReason)
	}
}

// TestDetectReleaseBuild release 产物在非容器环境下应当放行，
// 并带上平台归档名与可执行文件路径。
func TestDetectReleaseBuild(t *testing.T) {
	if InContainer() {
		t.Skip("running inside a container; self-update is intentionally blocked there")
	}

	restore := version.BuildType
	version.BuildType = version.BuildTypeRelease
	t.Cleanup(func() { version.BuildType = restore })

	c := Detect()
	if !c.CanUpdate {
		t.Fatalf("release build should be updatable, blocked by %q", c.BlockReason)
	}
	if c.ExecPath == "" {
		t.Error("ExecPath should be populated when update is allowed")
	}
	if c.AssetName != PlatformAssetSuffix() {
		t.Errorf("AssetName = %q, want %q", c.AssetName, PlatformAssetSuffix())
	}
	if c.Deployment != DeploymentBinary {
		t.Errorf("Deployment = %q, want binary", c.Deployment)
	}
	// 没有 systemd 托管时不能自动重启——退出即停服
	if c.Supervisor == SupervisorNone && c.CanRestart {
		t.Error("CanRestart must be false without a supervisor")
	}
}

// TestRestartConfigRelaunches 自更新重启以非零码退出，能否被拉起由 Restart= 加两个
// 退出码覆盖项共同决定。注意 systemd 的 Restart= 默认值是 no，不是 on-failure —— 因此
// 空值只可能来自查询失败，绝不能当成「有默认策略兜底」。
func TestRestartConfigRelaunches(t *testing.T) {
	cfg := func(restart string, success, prevent []int) unitRestartConfig {
		c := unitRestartConfig{
			Restart:                  restart,
			SuccessExitStatus:        map[int]bool{},
			RestartPreventExitStatus: map[int]bool{},
		}
		for _, n := range success {
			c.SuccessExitStatus[n] = true
		}
		for _, n := range prevent {
			c.RestartPreventExitStatus[n] = true
		}
		return c
	}

	cases := []struct {
		name string
		cfg  unitRestartConfig
		want bool
	}{
		{"always", cfg("always", nil, nil), true},
		{"on-failure", cfg("on-failure", nil, nil), true},
		// 只认 clean exit，非零退出码不管
		{"on-success", cfg("on-success", nil, nil), false},
		// 只认信号 / 超时 / watchdog，同样不认退出码
		{"on-abnormal", cfg("on-abnormal", nil, nil), false},
		{"on-abort", cfg("on-abort", nil, nil), false},
		{"on-watchdog", cfg("on-watchdog", nil, nil), false},
		{"no", cfg("no", nil, nil), false},
		{"empty means query failed", cfg("", nil, nil), false},

		// SuccessExitStatus=70 把自更新退出变成「成功退出」，on-failure 便不再拉起
		{"success status neutralizes on-failure", cfg("on-failure", []int{RestartExitCode}, nil), false},
		{"success status enables on-success", cfg("on-success", []int{RestartExitCode}, nil), true},
		{"unrelated success status", cfg("on-failure", []int{75}, nil), true},

		// RestartPreventExitStatus 一票否决，压过任何策略
		{"prevent overrides always", cfg("always", nil, []int{RestartExitCode}), false},
		{"prevent overrides on-failure", cfg("on-failure", nil, []int{RestartExitCode}), false},
		{"unrelated prevent status", cfg("always", nil, []int{75}), true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := restartConfigRelaunches(c.cfg); got != c.want {
				t.Errorf("restartConfigRelaunches(%+v) = %v, want %v", c.cfg, got, c.want)
			}
		})
	}
}

// TestParseExitStatusSet systemd 的退出码列表里可能混有信号名，只取得出数字的部分。
func TestParseExitStatusSet(t *testing.T) {
	got := parseExitStatusSet("70 75 SIGTERM")
	if !got[70] || !got[75] {
		t.Errorf("parseExitStatusSet lost numeric statuses: %v", got)
	}
	if len(got) != 2 {
		t.Errorf("parseExitStatusSet = %v, want exactly {70,75}", got)
	}
	if len(parseExitStatusSet("")) != 0 {
		t.Error("empty value should yield an empty set")
	}
}

// TestRestartRequestedDefaultsFalse 普通的 systemctl stop / Ctrl-C 必须退 0。
// 若这里默认为 true，正常停机会被记成 failed 并被 Restart=always 反复拉起。
func TestRestartRequestedDefaultsFalse(t *testing.T) {
	if RestartRequested() {
		t.Error("RestartRequested must be false unless Restart() was called")
	}
}

// TestRestartExitCodeIsNonZero 退出码为 0 会被 systemd 判定为 clean exit，
// Restart=on-failure 不会拉起 —— 这正是本次修复要消除的停服路径。
func TestRestartExitCodeIsNonZero(t *testing.T) {
	if RestartExitCode == 0 {
		t.Fatal("RestartExitCode must be non-zero, otherwise Restart=on-failure will not relaunch")
	}
}

func TestDirWritable(t *testing.T) {
	dir := t.TempDir()
	if !dirWritable(dir) {
		t.Error("temp dir should be writable")
	}

	if os.Geteuid() == 0 {
		t.Skip("running as root; permission bits do not restrict access")
	}
	readonly := filepath.Join(dir, "readonly")
	if err := os.Mkdir(readonly, 0o500); err != nil {
		t.Fatal(err)
	}
	if dirWritable(readonly) {
		t.Error("0500 dir should not be writable")
	}
}

func TestBackupPath(t *testing.T) {
	if got := backupPath("/opt/dujiao/dujiao-next"); got != "/opt/dujiao/dujiao-next.backup" {
		t.Errorf("backupPath = %q", got)
	}
}

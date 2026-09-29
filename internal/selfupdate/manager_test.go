package selfupdate

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/dujiao-next/internal/version"
)

// TestManagerStartRejectsUnsupportedEnv 环境不支持时必须在发起任何网络请求前拒绝。
func TestManagerStartRejectsUnsupportedEnv(t *testing.T) {
	restore := version.BuildType
	version.BuildType = version.BuildTypeSource
	t.Cleanup(func() { version.BuildType = restore })

	m := NewManager()
	err := m.Start(context.Background())
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("Start error = %v, want ErrNotSupported", err)
	}
	if m.Running() {
		t.Error("no task should be running after a rejected start")
	}
	if got := m.Snapshot().Status; got != StatusIdle {
		t.Errorf("status = %q, want idle", got)
	}
}

func TestManagerInitialSnapshot(t *testing.T) {
	m := NewManager()
	s := m.Snapshot()
	if s.Status != StatusIdle || s.Stage != StageIdle {
		t.Errorf("initial snapshot = %+v, want idle/idle", s)
	}
	if s.RestartRequired {
		t.Error("fresh manager should not require restart")
	}
}

// TestManagerRollbackWithoutBackup 没有备份时回滚要给出明确错误，而不是静默成功。
func TestManagerRollbackWithoutBackup(t *testing.T) {
	execPath, err := ExecutablePath()
	if err != nil {
		t.Skipf("cannot resolve executable path: %v", err)
	}
	if _, err := os.Stat(backupPath(execPath)); err == nil {
		t.Skip("a real backup exists next to the test binary")
	}

	m := NewManager()
	if err := m.Rollback(false); !errors.Is(err, ErrNoBackup) {
		t.Fatalf("Rollback error = %v, want ErrNoBackup", err)
	}
}

// TestManagerReportUpdatesProgress 进度回调要如实反映在可轮询的快照里。
func TestManagerReportUpdatesProgress(t *testing.T) {
	m := NewManager()
	m.report(StageDownloading, 42)

	s := m.Snapshot()
	if s.Stage != StageDownloading || s.Percent != 42 {
		t.Errorf("snapshot = %+v, want downloading/42", s)
	}
}

// TestManagerStartHoldsBinaryLockDuringReleaseFetch 复现 HTTP 检查更新与 CLI 回滚并发。
// 跨进程锁必须在网络请求之前取得，否则 CLI 会先消耗 backup，随后 HTTP 升级又继续替换。
func TestManagerStartHoldsBinaryLockDuringReleaseFetch(t *testing.T) {
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion: "v0.9.0",
		TargetVersion:   version.Version,
	})

	fetchEntered := make(chan struct{})
	releaseFetch := make(chan struct{})
	m := NewManager()
	m.detect = func() Capability {
		return Capability{CanUpdate: true, ExecPath: exec}
	}
	m.fetchLatest = func(context.Context) (*version.Release, error) {
		close(fetchEntered)
		<-releaseFetch
		// 与当前版本相同，让 Start 在释放锁后走 ErrNoUpdateAvailable，
		// 不需要真的发起下载。
		return &version.Release{TagName: version.Version}, nil
	}

	startDone := make(chan error, 1)
	go func() {
		startDone <- m.Start(context.Background())
	}()
	<-fetchEntered

	if err := rollbackAt(exec, false); !errors.Is(err, ErrUpdateInProgress) {
		t.Fatalf("rollback during release fetch = %v, want ErrUpdateInProgress", err)
	}

	close(releaseFetch)
	if err := <-startDone; !errors.Is(err, ErrNoUpdateAvailable) {
		t.Fatalf("Start = %v, want ErrNoUpdateAvailable", err)
	}
	// 网络检查结束后锁必须释放，不能永久阻断后续运维。
	if err := rollbackAt(exec, false); err != nil {
		t.Fatalf("rollback after release fetch completed = %v", err)
	}
}

// TestManagerStartRejectsMalformedReleaseVersion 自动替换二进制必须 fail-closed；
// 畸形 tag 不能因为与当前字符串不同就被当成“更新”。
func TestManagerStartRejectsMalformedReleaseVersion(t *testing.T) {
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion: "v1.2.2",
		TargetVersion:   "v1.2.3",
	})
	restoreVersion := version.Version
	version.Version = "v1.2.3"
	t.Cleanup(func() { version.Version = restoreVersion })

	m := NewManager()
	m.detect = func() Capability {
		return Capability{CanUpdate: true, ExecPath: exec}
	}
	m.fetchLatest = func(context.Context) (*version.Release, error) {
		return &version.Release{TagName: "v1.2"}, nil
	}

	err := m.Start(context.Background())
	if err == nil || errors.Is(err, ErrNoUpdateAvailable) {
		t.Fatalf("Start malformed release = %v, want a version parse error", err)
	}
	if m.Running() {
		t.Error("manager must release its busy state after rejecting a malformed release")
	}
	// 版本错误路径也必须释放 flock。
	unlock, lockErr := acquireBinaryLock(exec)
	if lockErr != nil {
		t.Fatalf("binary lock leaked after malformed release: %v", lockErr)
	}
	unlock()
}

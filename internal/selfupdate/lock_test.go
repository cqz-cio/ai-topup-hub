package selfupdate

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestBinaryLockIsExclusive 第二次获取必须失败而不是阻塞等待——
// 用户点了「回滚」，我们要立刻告诉他有别的操作在跑，而不是把 HTTP 请求挂住。
func TestBinaryLockIsExclusive(t *testing.T) {
	exec := filepath.Join(t.TempDir(), "dujiao-next")

	unlock, err := acquireBinaryLock(exec)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	if _, err := acquireBinaryLock(exec); !errors.Is(err, ErrUpdateInProgress) {
		t.Errorf("second acquire = %v, want ErrUpdateInProgress", err)
	}

	unlock()

	// 释放之后必须能重新拿到，否则一次失败的升级会把功能永久锁死
	unlock2, err := acquireBinaryLock(exec)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	unlock2()
}

// TestConcurrentRollbacksDoNotBothSucceed 两个并发回滚只能有一个成功。
// 若两个都跑，第二个会在第一个已经把 .backup 消耗掉之后继续操作同一批文件，
// 可能把唯一一份可恢复的二进制弄丢。
//
// 注意：go test -race 发现不了这个问题——它不是内存数据竞争，
// 而是多个流程对同一组文件做 rename 的事务竞争。
func TestConcurrentRollbacksDoNotBothSucceed(t *testing.T) {
	exec := filepath.Join(t.TempDir(), "dujiao-next")
	if err := os.WriteFile(exec, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath(exec), []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeBackupInfo(exec, BackupInfo{
		PreviousVersion: "v1.4.0",
		TargetVersion:   "v1.5.0",
	}); err != nil {
		t.Fatal(err)
	}

	const goroutines = 8
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		successes int
	)
	start := make(chan struct{})
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := rollbackAt(exec, false); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()

	if successes != 1 {
		t.Errorf("%d concurrent rollbacks succeeded, want exactly 1", successes)
	}
	if data, _ := os.ReadFile(exec); string(data) != "old-binary" {
		t.Errorf("binary = %q, want old-binary", data)
	}
	// 中间态文件不能留下
	if _, err := os.Stat(exec + ".rollback-tmp"); !os.IsNotExist(err) {
		t.Error("rollback stash file must not be left behind")
	}
}

// TestManagerRollbackHoldsBusyForWholeOperation Manager 的独占权必须覆盖整个回滚，
// 而不是「检查一下 running 就放开」——后者挡不住并发回滚，也挡不住刚起步的升级。
func TestManagerRollbackHoldsBusyForWholeOperation(t *testing.T) {
	m := NewManager()

	if err := m.acquire(); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	// 已被占用时回滚必须立即被拒
	if err := m.Rollback(false); !errors.Is(err, ErrUpdateInProgress) {
		t.Errorf("Rollback while busy = %v, want ErrUpdateInProgress", err)
	}
	m.releaseBusy()

	if m.Running() {
		t.Error("releaseBusy should clear the busy flag")
	}
}

// TestManagerAcquireIsExclusive 第二次 acquire 必须失败，这是 Start/Rollback 共用的闸门
func TestManagerAcquireIsExclusive(t *testing.T) {
	m := NewManager()
	if err := m.acquire(); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if err := m.acquire(); !errors.Is(err, ErrUpdateInProgress) {
		t.Errorf("second acquire = %v, want ErrUpdateInProgress", err)
	}
	m.releaseBusy()
	if err := m.acquire(); err != nil {
		t.Errorf("acquire after release: %v", err)
	}
	m.releaseBusy()
}

//go:build linux || darwin

package selfupdate

import (
	"fmt"
	"os"
	"syscall"
)

// acquireBinaryLock 对二进制取一把跨进程独占锁，拿不到时返回 ErrUpdateInProgress。
//
// Manager 的进程内互斥挡不住「后台点升级」与「终端跑 dujiao-next rollback」同时发生：
// 那是两个进程，共享的只有磁盘上的 exec / .backup / .rollback-tmp 这几个文件。
// 交错执行足以把最后一份可恢复的备份弄丢。
//
// 用 flock 而不是「创建独占文件」：flock 由内核在进程退出时自动释放，
// 崩溃或被 kill 不会留下一把谁都解不开的死锁。
func acquireBinaryLock(execPath string) (func(), error) {
	f, err := os.OpenFile(execPath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open update lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, ErrUpdateInProgress
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

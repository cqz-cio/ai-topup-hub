package selfupdate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/dujiao-next/internal/version"
)

// BackupInfo 一次升级留下的元数据，与 <exec>.backup 同目录持久化。
//
// 为什么要落盘：Manager 的状态只活在内存里，进程一重启就没了，之后只剩一个
// 光秃秃的 .backup 文件——既不知道它是哪个版本，也不知道新版本到底起没起来过。
// 而「新版本是否已经开始迁移、有没有成功启动」恰恰是判断回滚安不安全的依据：
// 迁移只要开始，就可能已经部分推进 schema；即使最后启动失败，也不能再把它当成
// 可以无确认回滚的干净窗口。
type BackupInfo struct {
	// PreviousVersion 备份里那个二进制的版本，也就是回滚后会跑的版本
	PreviousVersion string `json:"previous_version"`
	// TargetVersion 本次升级到的版本
	TargetVersion string `json:"target_version"`
	// SwappedAt 二进制完成替换的时间
	SwappedAt time.Time `json:"swapped_at"`
	// MigrationStarted 新版本是否已经进入数据库迁移阶段。
	// 一旦为 true，即使迁移后来报错，也必须把回滚视为不安全。
	MigrationStarted bool `json:"migration_started"`
	// MigrationStartedAt 首次进入数据库迁移阶段的时间
	MigrationStartedAt *time.Time `json:"migration_started_at,omitempty"`
	// NewVersionStarted 新版本是否已经完整启动过一次。
	// 这是展示与诊断信息；真正的安全闸门同时检查 MigrationStarted。
	NewVersionStarted bool `json:"new_version_started"`
	// NewVersionStartedAt 新版本首次成功启动的时间
	NewVersionStartedAt *time.Time `json:"new_version_started_at,omitempty"`
}

// metadataPath 备份元数据的落盘路径，跟着备份文件走
func metadataPath(execPath string) string {
	return backupPath(execPath) + ".json"
}

// rollbackMarker 记录一次已经完成的磁盘回滚。
//
// 为什么还需要它：systemd 可能已经把新二进制加载进内存，同时运维人员在另一个
// 进程里完成 CLI 回滚。磁盘上虽然已经恢复旧版本，那个尚未开始迁移的新进程仍可能
// 继续运行。它启动时看到这个标记，就必须在碰数据库之前退出。
type rollbackMarker struct {
	SupersededVersion string    `json:"superseded_version"`
	RolledBackAt      time.Time `json:"rolled_back_at"`
}

func rollbackMarkerPath(execPath string) string {
	return execPath + ".rollback.json"
}

// ErrStartupSuperseded 表示当前已加载进内存的版本刚被另一个进程回滚，
// 当前进程必须在数据库初始化与迁移之前退出。
var ErrStartupSuperseded = errors.New("current binary was superseded by a completed rollback")

func writeJSONAtomically(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepathBase(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", filepathBase(path), err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit %s: %w", filepathBase(path), err)
	}
	return nil
}

// filepathBase 只用于错误信息，避免把部署绝对路径完整写进低层错误。
func filepathBase(path string) string {
	if i := strings.LastIndexByte(path, os.PathSeparator); i >= 0 {
		return path[i+1:]
	}
	return path
}

// writeBackupInfo 落盘升级元数据。先写临时文件再 rename，避免进程在写一半时被杀
// 留下半截 JSON——那会让后续所有读取都失败，等于丢失回滚信息。
func writeBackupInfo(execPath string, info BackupInfo) error {
	return writeJSONAtomically(metadataPath(execPath), info)
}

func writeRollbackMarker(execPath, supersededVersion string) error {
	return writeJSONAtomically(rollbackMarkerPath(execPath), rollbackMarker{
		SupersededVersion: strings.TrimSpace(supersededVersion),
		RolledBackAt:      time.Now(),
	})
}

func readRollbackMarker(execPath string) (rollbackMarker, bool, error) {
	data, err := os.ReadFile(rollbackMarkerPath(execPath))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return rollbackMarker{}, false, nil
		}
		return rollbackMarker{}, false, fmt.Errorf("read rollback marker: %w", err)
	}
	var marker rollbackMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return rollbackMarker{}, false, fmt.Errorf("decode rollback marker: %w", err)
	}
	if strings.TrimSpace(marker.SupersededVersion) == "" {
		return rollbackMarker{}, false, errors.New("rollback marker has no superseded version")
	}
	return marker, true, nil
}

func removeRollbackMarker(execPath string) {
	_ = os.Remove(rollbackMarkerPath(execPath))
}

// ReadBackupInfo 读取升级元数据。
//
// 第二个返回值 known 表示元数据是否可信。以下情况都是 known=false：
//   - 元数据文件不存在。最典型的就是**从旧版本升上来的第一次**：执行替换的是旧进程，
//     它根本没有写元数据这段代码，落盘的只有一个 .backup。
//   - JSON 损坏（写到一半掉电、磁盘满）。
//
// 调用方必须把 known=false 当作「不知道新版本跑没跑过」，而不是「没跑过」。
// 这两者的差别就是回滚闸门到底有没有用：判成「没跑过」会直接放行回滚，
// 而现实中这些实例的数据库迁移很可能早就执行过了。
func ReadBackupInfo(execPath string) (info BackupInfo, known bool, err error) {
	data, readErr := os.ReadFile(metadataPath(execPath))
	if readErr != nil {
		if errors.Is(readErr, fs.ErrNotExist) {
			return BackupInfo{}, false, nil
		}
		return BackupInfo{}, false, fmt.Errorf("read backup info: %w", readErr)
	}
	if jsonErr := json.Unmarshal(data, &info); jsonErr != nil {
		return BackupInfo{}, false, nil
	}
	if info.TargetVersion == "" {
		// 结构合法但没有目标版本，同样无法据此判断安全性
		return info, false, nil
	}
	return info, true, nil
}

// removeBackupInfo 清理元数据，回滚完成后调用
func removeBackupInfo(execPath string) {
	_ = os.Remove(metadataPath(execPath))
}

// rollbackUnsafe 是所有回滚入口的唯一安全判断，避免 HTTP、CLI 与前端各自推导。
func rollbackUnsafe(info BackupInfo, known bool) bool {
	return !known || info.MigrationStarted || info.NewVersionStarted
}

// StartupGuard 把一次新版本启动与 CLI/HTTP 回滚串行化。
//
// guard 在正常启动尽可能早的阶段取得，与 rollbackAt 使用同一把 flock，并一直持有到
// 数据库迁移完成。这样回滚只有两个结果：要么在迁移开始前先完成，当前新进程看到
// rollback marker 后退出；要么迁移先占住锁，回滚立即得到 ErrUpdateInProgress。
type StartupGuard struct {
	execPath       string
	currentVersion string
	info           BackupInfo
	known          bool
	active         bool
	unlock         func()
	released       bool
}

// AcquireStartupGuard 为正式发布二进制取得启动闸门。源码构建从未参与自更新，
// 直接返回空 guard，避免在 go run / 测试二进制旁创建无意义的锁文件。
func AcquireStartupGuard(currentVersion string) (*StartupGuard, error) {
	if !version.IsReleaseBuild() {
		return &StartupGuard{}, nil
	}
	execPath, err := ExecutablePath()
	if err != nil {
		return nil, err
	}
	return acquireStartupGuardAt(execPath, currentVersion)
}

// acquireStartupGuardAt 是可测实现。
func acquireStartupGuardAt(execPath, currentVersion string) (*StartupGuard, error) {
	// 绝大多数启动都不处在升级/回滚窗口，不应为了一个不存在的备份去创建
	// <exec>.lock。正式二进制完全可以从只读目录运行；CanUpdate=false 只应禁用
	// 自更新，不能反过来让服务本身无法启动。
	needsGuard := false
	for _, path := range []string{backupPath(execPath), rollbackMarkerPath(execPath)} {
		if _, err := os.Stat(path); err == nil {
			needsGuard = true
			break
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("inspect startup update state: %w", err)
		}
	}
	if !needsGuard {
		return &StartupGuard{}, nil
	}

	unlock, err := acquireBinaryLock(execPath)
	if err != nil {
		return nil, err
	}
	releaseOnError := func(err error) (*StartupGuard, error) {
		unlock()
		return nil, err
	}

	if marker, exists, err := readRollbackMarker(execPath); err != nil {
		return releaseOnError(err)
	} else if exists {
		if strings.TrimSpace(marker.SupersededVersion) == strings.TrimSpace(currentVersion) {
			// 标记只拦截一个已经加载、但尚未进入启动闸门的旧进程。命中后立即消费：
			// 回滚后的版本可能来自尚无 StartupGuard 的历史发行版，无法替我们清理标记；
			// 若永久保留，用户以后重新升级到同一个版本会被历史回滚无限阻断。
			//
			// systemd unit 同一时刻只会启动一个主进程；这个进程退出后，磁盘上的回滚
			// 版本才会被拉起。即使当时根本没有竞态，后续重装同版本也最多安全地多退出一次。
			removeRollbackMarker(execPath)
			return releaseOnError(ErrStartupSuperseded)
		}
		// 当前运行的是回滚后的版本，旧 marker 已完成它的使命。
		removeRollbackMarker(execPath)
	}

	if _, err := os.Stat(backupPath(execPath)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			unlock()
			return &StartupGuard{}, nil
		}
		return releaseOnError(fmt.Errorf("stat backup before startup: %w", err))
	}

	info, known, err := ReadBackupInfo(execPath)
	if err != nil {
		return releaseOnError(err)
	}
	if known && strings.TrimSpace(info.TargetVersion) != strings.TrimSpace(currentVersion) {
		// 这份元数据描述的是另一个版本，用它判断当前二进制没有任何依据。
		//
		// 绝不能据此拒绝启动。最常见的来源是文档里那条手工升级流程：用户只替换
		// dujiao-next，上一次一键升级留下的 .backup / .backup.json 原封不动留在目录里。
		// .backup 会无限期存在，把「元数据目标对不上」当成「当前版本已被回滚取代」，
		// 等于每次手工升级之后服务永久起不来，而日志里只会说自己被回滚了。
		//
		// 真正的取代信号是 .rollback.json，已经在上面单独处理过。
		// 这里按 unknown 处理：启动继续，MarkMigrationStarted 会用描述当前二进制的
		// 保守记录覆盖它，回滚闸门照样 fail-closed（PreviousVersion 一并丢弃 ——
		// 手工替换过二进制的目录里，.backup 是不是元数据说的那一版已经无法证明）。
		info, known = BackupInfo{}, false
	}

	return &StartupGuard{
		execPath:       execPath,
		currentVersion: currentVersion,
		info:           info,
		known:          known,
		active:         true,
		unlock:         unlock,
	}, nil
}

// MarkMigrationStarted 必须紧挨在 AutoMigrate 之前调用。状态持久化失败时调用方
// 必须停止启动，绝不能在元数据仍显示“安全”的情况下继续修改数据库。
func (g *StartupGuard) MarkMigrationStarted() error {
	if g == nil || !g.active || g.released {
		return nil
	}
	if g.info.MigrationStarted {
		return nil
	}

	now := time.Now()
	if !g.known {
		g.info = BackupInfo{
			TargetVersion: g.currentVersion,
			SwappedAt:     now,
		}
	}
	g.info.MigrationStarted = true
	g.info.MigrationStartedAt = &now
	if err := writeBackupInfo(g.execPath, g.info); err != nil {
		return fmt.Errorf("persist migration-started state: %w", err)
	}
	g.known = true
	return nil
}

// MarkStartupCompleted 在数据库迁移与启动初始化完成后补充诊断状态。
// 即使这里失败，MigrationStarted 已经持久化，回滚仍会 fail-closed。
func (g *StartupGuard) MarkStartupCompleted() error {
	if g == nil || !g.active || g.released {
		return nil
	}
	if err := g.MarkMigrationStarted(); err != nil {
		return err
	}
	if g.info.NewVersionStarted {
		return nil
	}
	now := time.Now()
	g.info.NewVersionStarted = true
	g.info.NewVersionStartedAt = &now
	if err := writeBackupInfo(g.execPath, g.info); err != nil {
		return fmt.Errorf("persist startup-completed state: %w", err)
	}
	return nil
}

// Release 释放启动期间持有的跨进程锁；可重复调用。
func (g *StartupGuard) Release() {
	if g == nil || g.released {
		return
	}
	g.released = true
	if g.unlock != nil {
		g.unlock()
	}
}

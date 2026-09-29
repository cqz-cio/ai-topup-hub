package selfupdate

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dujiao-next/internal/version"
)

// Stage 升级过程中的阶段，用于前端展示进度条文案
type Stage string

const (
	StageIdle        Stage = "idle"
	StageDownloading Stage = "downloading"
	StageVerifying   Stage = "verifying"
	StageExtracting  Stage = "extracting"
	StageSwapping    Stage = "swapping"
	StageDone        Stage = "done"
)

// Status 升级任务的整体状态
type Status string

const (
	StatusIdle      Status = "idle"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

// ErrUpdateInProgress 已有升级任务在执行
var ErrUpdateInProgress = errors.New("update already in progress")

// State 升级任务状态快照，直接序列化给前端轮询
type State struct {
	Status  Status `json:"status"`
	Stage   Stage  `json:"stage"`
	Percent int    `json:"percent"`
	// TargetVersion 本次升级的目标版本号
	TargetVersion string `json:"target_version,omitempty"`
	// Error 失败原因（英文技术细节），前端按 status 决定是否展示
	Error string `json:"error,omitempty"`
	// RestartRequired 二进制已替换但进程仍是旧版本，需要重启才生效
	RestartRequired bool       `json:"restart_required"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

// Manager 串行化升级任务并维护可轮询的状态。
// 升级过程跨越单次 HTTP 请求（下载可能几分钟），因此任务在后台 goroutine 执行，
// 前端通过 Snapshot 轮询进度。
type Manager struct {
	mu      sync.Mutex
	state   State
	running bool
	updater *Updater
	// now 便于测试注入时间
	now func() time.Time
	// detect / fetchLatest 便于把环境与网络边界注入测试，尤其用于复现
	// 「HTTP 正在检查更新时 CLI 回滚」这种跨进程时序。
	detect      func() Capability
	fetchLatest func(context.Context) (*version.Release, error)
}

// NewManager 创建升级任务管理器
func NewManager() *Manager {
	return &Manager{
		state:   State{Status: StatusIdle, Stage: StageIdle},
		updater: NewUpdater(),
		now:     time.Now,
		detect:  Detect,
		fetchLatest: func(ctx context.Context) (*version.Release, error) {
			return version.FetchLatestRelease(ctx)
		},
	}
}

// Snapshot 返回当前状态副本
func (m *Manager) Snapshot() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// Start 校验环境后异步执行升级。
// 返回 nil 表示任务已成功启动，不代表升级完成——完成情况需轮询 Snapshot。
func (m *Manager) Start(ctx context.Context) error {
	c := m.detect()
	if !c.CanUpdate {
		return ErrNotSupported
	}

	// 必须在拉 release 之前就占坑。之前是拉完才置 running，中间那段网络等待里
	// running 仍是 false，一个并发的回滚检查会顺利通过，然后两条流程同时对
	// exec / .backup / .rollback-tmp 做 rename。
	if err := m.acquire(); err != nil {
		return err
	}

	execPath := c.ExecPath
	if execPath == "" {
		var err error
		execPath, err = ExecutablePath()
		if err != nil {
			m.releaseBusy()
			return err
		}
	}

	// 跨进程锁也必须在拉 release 之前取得。Manager.running 只能拦住当前
	// HTTP 进程里的另一个请求，拦不住终端里单独启动的 `dujiao-next rollback`。
	// 锁一直交给异步 Apply 持有，覆盖检查、下载、替换和元数据落盘的完整事务。
	unlock, err := acquireBinaryLock(execPath)
	if err != nil {
		m.releaseBusy()
		return err
	}

	release, err := m.fetchLatest(ctx)
	if err != nil {
		unlock()
		m.releaseBusy()
		return err
	}
	hasUpdate, compareErr := version.IsNewerVersion(release.TagName, version.Version)
	if compareErr != nil {
		unlock()
		m.releaseBusy()
		return fmt.Errorf("compare release version %q with current %q: %w",
			release.TagName, version.Version, compareErr)
	}
	if !hasUpdate {
		unlock()
		m.releaseBusy()
		return ErrNoUpdateAvailable
	}

	m.mu.Lock()
	startedAt := m.now()
	m.state = State{
		Status:        StatusRunning,
		Stage:         StageDownloading,
		TargetVersion: release.TagName,
		StartedAt:     &startedAt,
	}
	m.mu.Unlock()

	// 不继承请求 ctx：HTTP 响应一返回请求就被取消，会把下载一并掐掉。
	// 用独立 ctx 并以 downloadTimeout 兜底。
	go m.run(release, execPath, unlock)
	return nil
}

// acquire 取得「正在改动二进制」的独占权。升级与回滚共用同一把锁 ——
// 它们操作的是同一组文件（exec、.backup、.rollback-tmp），任何交错都可能
// 把最后一份可恢复的备份弄丢。
func (m *Manager) acquire() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return ErrUpdateInProgress
	}
	m.running = true
	return nil
}

// releaseBusy 释放独占权
func (m *Manager) releaseBusy() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
}

func (m *Manager) run(release *version.Release, execPath string, unlock func()) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	// 无论 Apply 成功还是失败，都在修改状态前先释放跨进程锁；调用方随后
	// 发起回滚时看到的是已经落定的二进制与元数据，而不是半完成现场。
	err := func() error {
		defer unlock()
		return m.updater.applyLocked(ctx, release, execPath, m.report)
	}()

	m.mu.Lock()
	defer m.mu.Unlock()
	finishedAt := m.now()
	m.state.FinishedAt = &finishedAt
	m.running = false
	if err != nil {
		m.state.Status = StatusFailed
		m.state.Error = err.Error()
		return
	}
	m.state.Status = StatusSucceeded
	m.state.Stage = StageDone
	m.state.Percent = 100
	m.state.RestartRequired = true
}

func (m *Manager) report(stage Stage, percent int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Stage = stage
	m.state.Percent = percent
}

// Rollback 还原上一版本二进制。升级任务执行期间拒绝回滚，避免与替换过程交错。
//
// force 为 false 时，若数据库迁移已经开始、或升级元数据不可信，会返回
// ErrRollbackUnsafe，由前端弹出风险确认后再带 force 重试。
func (m *Manager) Rollback(force bool) error {
	// 整个回滚期间都持有独占权，而不是「检查一下就放开」——
	// 后者挡不住两个并发回滚，也挡不住一个刚起步的升级。
	if err := m.acquire(); err != nil {
		return err
	}
	defer m.releaseBusy()

	if err := m.updater.Rollback(force); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.RestartRequired = true
	return nil
}

// Running 是否有升级任务在执行
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

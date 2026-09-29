package selfupdate

import (
	"os"
	"sync/atomic"
	"syscall"
	"time"
)

// restartDelay 触发退出前的等待时间，保证 HTTP 响应已经写回客户端。
// 前端需要先拿到「正在重启」的结果才能进入轮询等待状态。
const restartDelay = 500 * time.Millisecond

// RestartExitCode 自更新重启专用退出码。
//
// 关键点：systemd 把「收到 SIGTERM 后正常退出」视为 clean exit，而官方部署文档里的
// Restart=on-failure 对 clean exit 不做任何处理 —— 进程停了就停了，服务直接下线。
// 所以自更新触发的退出必须以非零码结束，才能被 on-failure / always 两种策略拉起。
//
// 取 70（EX_SOFTWARE）而不是 1，是为了在 journal 里能一眼把「自更新重启」和
// 真正的启动失败区分开；同时它不能出现在 SuccessExitStatus= 中，否则又会被当成 clean exit。
const RestartExitCode = 70

// restartRequested 标记本次退出是自更新重启，而不是 systemctl stop / Ctrl-C。
// 二者都走 SIGTERM，只能靠这个标记区分：前者要以非零码退出换取 systemd 拉起，
// 后者必须老老实实退 0，否则 systemctl stop 会把服务标成 failed 并被 always 策略反复拉起。
var restartRequested atomic.Bool

// RestartRequested 报告当前退出是否由自更新重启触发。
// 由 cmd/server 在优雅关闭结束后调用，决定最终的进程退出码。
func RestartRequested() bool {
	return restartRequested.Load()
}

// Restart 让当前进程退出，由 systemd 按 Restart= 策略拉起新二进制。
//
// 走 SIGTERM 而不是 os.Exit：cmd/server 用 signal.NotifyContext 监听 SIGTERM，
// 收到后会依次 Stop 各个 Service —— HTTP server 优雅关闭、asynq worker 等待在途任务。
// 直接 os.Exit 会让正在处理的订单请求和后台任务被硬切断。
//
// 优雅关闭走完后，cmd/server 会读 RestartRequested() 并以 RestartExitCode 退出。
//
// 调用前必须确认 Detect().CanRestart 为 true。没有守护进程、或 unit 的 Restart=
// 策略不会因非零退出码拉起时，退出即停服，应由前端提示用户手动重启。
func Restart() {
	restartRequested.Store(true)
	go func() {
		time.Sleep(restartDelay)
		p, err := os.FindProcess(os.Getpid())
		if err != nil {
			// 理论上对自身 PID 不会失败；真失败了就直接退出，
			// 让 systemd 拉起新进程，代价是跳过优雅关闭。
			os.Exit(RestartExitCode)
			return
		}
		if err := p.Signal(syscall.SIGTERM); err != nil {
			os.Exit(RestartExitCode)
		}
	}()
}

package contract

import "time"

// Queue 是订单应用层使用的异步任务端口。
type Queue interface {
	Enabled() bool
	EnqueueTimeoutCancel(orderID uint, delay time.Duration) error
	EnqueueStatusEmail(orderID uint, status string) error
}

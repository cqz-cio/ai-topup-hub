package contract

import notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"

var (
	ErrNotifyConfigInvalid = notificationcontract.ErrConfigInvalid
	ErrNotifySendFailed    = notificationcontract.ErrSendFailed
)

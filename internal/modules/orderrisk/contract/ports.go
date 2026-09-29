package contract

import settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"

type SettingReader interface {
	GetOrderRiskControlConfig() (settingssecurity.OrderRiskControlConfig, error)
}

// PendingOrderGate 在订单事务中序列化相同风控身份并读取真实待支付库存占用。
type PendingOrderGate interface {
	LockRiskKeys(keys []string) error
	CountPendingByUserID(userID uint) (int64, error)
	CountPendingGuestByRiskIP(riskIP string) (int64, error)
	CountPendingMemberByRiskIP(riskIP string) (int64, error)
	SumPendingGuestQuantityByRiskIP(riskIP string, productIDs []uint) (map[uint]int64, error)
}

// RateLimiter 执行具有外部状态的下单频率限制。
type RateLimiter interface {
	Check(input CheckInput, config settingssecurity.OrderRateLimitConfig) error
}

// Controller 是订单上下文调用风控所需的用例端口。
type Controller interface {
	CheckOrderAllowed(input CheckInput) (CheckResult, error)
	CheckPendingOrderAllowed(input CheckInput, prepared CheckResult, gate PendingOrderGate) error
}

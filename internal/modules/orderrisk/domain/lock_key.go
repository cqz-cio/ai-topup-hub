package domain

import "time"

// LockKey 仅保存风控维度摘要，用于跨进程序列化相同身份的库存配额检查。
type LockKey struct {
	KeyHash   string    `gorm:"type:varchar(64);primaryKey" json:"-"`
	CreatedAt time.Time `gorm:"not null" json:"-"`
}

func (LockKey) TableName() string { return "order_risk_lock_keys" }

package domain

import "time"

const (
	BalanceWaitWaiting = "waiting"
	BalanceWaitReady   = "ready"
	BalanceWaitSent    = "sent"
	BalanceWaitClosed  = "closed"
)

// BalanceWait tracks one customer's blocked checkout, independently of the
// supplier-wide operator alert cooldown. It never authorizes a purchase.
type BalanceWait struct {
	ID            uint   `gorm:"primarykey"`
	OrderID       uint   `gorm:"uniqueIndex;not null"`
	OrderNo       string `gorm:"type:varchar(64);not null"`
	Recipient     string `gorm:"type:varchar(255);not null"`
	Products      string `gorm:"type:text;not null"`
	Requirements  string `gorm:"type:text;not null"`
	Status        string `gorm:"type:varchar(20);index;not null"`
	RecoveredAt   *time.Time
	SentAt        *time.Time
	LeaseToken    string `gorm:"type:varchar(64);not null;default:''"`
	LeaseUntil    *time.Time
	NextAttemptAt *time.Time `gorm:"index"`
	Attempts      int        `gorm:"not null;default:0"`
	LastError     string     `gorm:"type:varchar(255);not null;default:''"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (BalanceWait) TableName() string { return "supplier_balance_waits" }

type FundingRequirement struct {
	ConnectionID uint   `json:"connection_id"`
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
}

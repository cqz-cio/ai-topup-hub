package domain

import "time"

// ChainInvoice keeps the fiat price and exchange rate immutable. AmountKey is
// never reused, including after expiry, so late transfers cannot buy a new order.
type ChainInvoice struct {
	ID          uint      `gorm:"primaryKey"`
	PaymentID   uint      `gorm:"uniqueIndex;not null"`
	Stream      string    `gorm:"size:160;index;not null"`
	AmountKey   string    `gorm:"size:240;uniqueIndex;not null"`
	Units       string    `gorm:"size:80;not null"`
	Amount      string    `gorm:"size:80;not null"`
	FiatAmount  string    `gorm:"size:40;not null"`
	Currency    string    `gorm:"size:8;not null"`
	CNYPerUSDT  string    `gorm:"size:40;not null"`
	Recipient   string    `gorm:"size:42;not null"`
	StartBlock  uint64    `gorm:"not null"`
	ExpiresAt   time.Time `gorm:"index;not null"`
	CreatedAt   time.Time
	State       string  `gorm:"size:32;index;not null"`
	TransferKey *string `gorm:"size:100;uniqueIndex"`
}

func (ChainInvoice) TableName() string { return "chain_payment_invoices" }

type ChainCursor struct {
	Stream       string `gorm:"size:160;primaryKey"`
	NextBlock    uint64 `gorm:"not null"`
	PreviousHash string `gorm:"size:66"`
	UpdatedAt    time.Time
}

func (ChainCursor) TableName() string { return "chain_payment_cursors" }

// ChainTransfer doubles as a durable settlement outbox. A crash between order
// settlement and marking delivered replays the existing idempotent callback.
type ChainTransfer struct {
	Key           string `gorm:"size:100;primaryKey"`
	Stream        string `gorm:"size:160;index;not null"`
	TxHash        string `gorm:"size:66;index;not null"`
	LogIndex      uint64
	Block         uint64
	BlockHash     string `gorm:"size:66"`
	Sender        string `gorm:"size:42"`
	Units         string `gorm:"size:80"`
	PaidAt        time.Time
	PaymentID     uint       `gorm:"index"`
	State         string     `gorm:"size:32;index"`
	Reason        string     `gorm:"size:64"`
	NextAttemptAt *time.Time `gorm:"index"`
	CreatedAt     time.Time
}

func (ChainTransfer) TableName() string { return "chain_payment_transfers" }

package domain

import "time"

// Task stores one immutable recharge attempt for one paid leaf order.
// Session and verification data are encrypted; card credentials are never persisted.
type Task struct {
	ActiveAccountHash  *string    `gorm:"uniqueIndex;size:64" json:"-"`
	CodeHash           *string    `gorm:"uniqueIndex;size:64" json:"-"`
	CodeCipher         string     `gorm:"type:text" json:"-"`
	IssuedAt           *time.Time `json:"issued_at,omitempty"`
	CheckedUntil       *time.Time `json:"-"`
	AccountCipher      string     `gorm:"type:text" json:"-"`
	ConfirmationHash   string     `gorm:"size:64" json:"-"`
	ConfirmedAt        *time.Time `json:"confirmed_at,omitempty"`
	RedeemedAt         *time.Time `json:"redeemed_at,omitempty"`
	ID                 uint       `gorm:"primaryKey" json:"id"`
	OrderID            uint       `gorm:"uniqueIndex;not null" json:"order_id"`
	State              string     `gorm:"index;size:32;not null" json:"state"`
	Plan               string     `json:"plan"`
	Region             string     `json:"region"`
	RegionVersion      int        `json:"-"`
	Channel            string     `json:"-"`
	CardRule           string     `json:"-"`
	CardRef            string     `json:"-"`
	IdempotencyKey     string     `gorm:"uniqueIndex;size:36" json:"-"`
	OperationID        string     `gorm:"size:128" json:"-"`
	SessionCipher      string     `gorm:"type:text" json:"-"`
	VerificationCipher string     `gorm:"type:text" json:"-"`
	LastCode           string     `json:"code,omitempty"`
	BillingState       string     `json:"-"`
	ChargedCredits     int        `json:"-"`
	CancelAfterSuccess bool       `json:"-"`
	LeaseToken         string     `json:"-"`
	LeaseUntil         time.Time  `json:"-"`
	NextRunAt          time.Time  `gorm:"index" json:"-"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (Task) TableName() string { return "auto_recharge_tasks" }

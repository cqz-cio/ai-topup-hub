package contract

import (
	"context"
	"errors"
	"time"

	"github.com/dujiao-next/internal/modules/procurement/domain"
)

// A delivery accepted by SMTP must not be resent merely because audit logging failed.
var ErrBalanceRecoveryMailAccepted = errors.New("balance recovery email accepted but audit write failed")

type BalanceRecoveryOrder struct {
	OrderNo, Recipient, Products, Status string
	ExpiresAt, CanceledAt, PaidAt        *time.Time
	ResellerID                           *uint
}

type BalanceRecoveryOrderReader interface {
	ReadBalanceRecoveryOrder(context.Context, uint) (*BalanceRecoveryOrder, error)
}

type BalanceWaitRepository interface {
	CreateBalanceWait(context.Context, *domain.BalanceWait) error
	ListBalanceWaits(context.Context, uint, int, time.Time) ([]domain.BalanceWait, error)
	SetBalanceWaitReady(context.Context, uint, bool, time.Time) error
	CloseBalanceWait(context.Context, uint, string) error
	ClaimBalanceMail(context.Context, uint, string, time.Time) (*domain.BalanceWait, error)
	FinishBalanceMail(context.Context, uint, string, time.Time, bool) error
}

type BalanceRecoveryQueue interface {
	EnqueueBalanceRecoveryEmail(uint) error
}

type BalanceRecoveryMailer interface {
	SendBalanceRecoveryEmail(context.Context, *domain.BalanceWait) error
}

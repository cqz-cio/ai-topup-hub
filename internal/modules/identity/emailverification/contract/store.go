package emailverificationcontract

import (
	"time"

	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
)

type Store interface {
	Create(*emailverificationdomain.Code) error
	GetLatest(email, purpose string) (*emailverificationdomain.Code, error)
	MarkVerified(id uint, verifiedAt time.Time) error
	IncrementAttempt(id uint) error
}

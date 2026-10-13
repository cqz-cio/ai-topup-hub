package emailverificationcontract

import (
	"time"

	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
)

type Store interface {
	Create(*emailverificationdomain.Code) error
	GetLatest(email, purpose string) (*emailverificationdomain.Code, error)
	// Atomically counts an attempt and consumes a matching, latest, unexpired code.
	ApplyVerificationAttempt(id uint, matches bool, now time.Time, maxAttempts int) (bool, error)
}

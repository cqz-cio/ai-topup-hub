package emailverificationstore

import (
	"errors"
	"time"

	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Create(code *emailverificationdomain.Code) error {
	return store.db.Create(code).Error
}

func (store *Store) GetLatest(email, purpose string) (*emailverificationdomain.Code, error) {
	var record emailverificationdomain.Code
	if err := store.db.Where("email = ? AND purpose = ? AND deleted_at IS NULL", email, purpose).
		Order("sent_at desc, id desc").
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// ApplyVerificationAttempt performs the attempt budget check and optional
// consumption in one conditional UPDATE, including concurrent callers.
func (store *Store) ApplyVerificationAttempt(id uint, matches bool, now time.Time, maxAttempts int) (bool, error) {
	updates := map[string]interface{}{"attempt_count": gorm.Expr("attempt_count + 1")}
	if matches {
		updates["verified_at"] = now
	}
	result := store.db.Model(&emailverificationdomain.Code{}).
		Where("id = ? AND deleted_at IS NULL AND verified_at IS NULL AND expires_at > ? AND attempt_count < ?", id, now, maxAttempts).
		Where(`NOT EXISTS (SELECT 1 FROM email_verify_codes AS newer
            WHERE newer.email = email_verify_codes.email AND newer.purpose = email_verify_codes.purpose
            AND newer.deleted_at IS NULL AND (newer.sent_at > email_verify_codes.sent_at
            OR (newer.sent_at = email_verify_codes.sent_at AND newer.id > email_verify_codes.id)))`).
		Updates(updates)
	return result.RowsAffected == 1, result.Error
}

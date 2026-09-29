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

func (store *Store) MarkVerified(id uint, verifiedAt time.Time) error {
	return store.db.Model(&emailverificationdomain.Code{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("verified_at", verifiedAt).Error
}

func (store *Store) IncrementAttempt(id uint) error {
	return store.db.Model(&emailverificationdomain.Code{}).
		Where("id = ? AND deleted_at IS NULL", id).
		UpdateColumn("attempt_count", gorm.Expr("attempt_count + 1")).Error
}

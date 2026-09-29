package gormstore

import (
	"context"
	"errors"
	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	"github.com/dujiao-next/internal/modules/autorecharge/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) *Store { return &Store{db: db} }
func (s *Store) Ensure(ctx context.Context, t *domain.Task) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "order_id"}}, DoNothing: true}).Create(t).Error
}
func (s *Store) Get(ctx context.Context, orderID uint) (*domain.Task, error) {
	var t domain.Task
	err := s.db.WithContext(ctx).Where("order_id = ?", orderID).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, contract.ErrNotFound
	}
	return &t, err
}
func (s *Store) FindCode(ctx context.Context, hash string) (*domain.Task, error) {
	var t domain.Task
	err := s.db.WithContext(ctx).Where("code_hash = ? AND issued_at IS NOT NULL", hash).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, contract.ErrNotFound
	}
	return &t, err
}
func (s *Store) Due(ctx context.Context, now time.Time, limit int) ([]uint, error) {
	var ids []uint
	err := s.db.WithContext(ctx).Model(&domain.Task{}).Where("state NOT IN ? AND next_run_at <= ? AND lease_until < ?", []string{"completed", "failed", "canceled", "manual_review", "issued"}, now, now).Order("next_run_at, id").Limit(limit).Pluck("order_id", &ids).Error
	return ids, err
}
func (s *Store) Claim(ctx context.Context, orderID uint, now time.Time) (*domain.Task, error) {
	token := uuid.NewString()
	r := s.db.WithContext(ctx).Model(&domain.Task{}).Where("order_id = ? AND lease_until < ?", orderID, now).Updates(map[string]interface{}{"lease_token": token, "lease_until": now.Add(2 * time.Minute)})
	if r.Error != nil {
		return nil, r.Error
	}
	if r.RowsAffected != 1 {
		return nil, contract.ErrBusy
	}
	return s.Get(ctx, orderID)
}
func (s *Store) Save(ctx context.Context, t *domain.Task) error {
	token := t.LeaseToken
	t.LeaseToken = ""
	t.LeaseUntil = time.Time{}
	t.UpdatedAt = time.Now().UTC()
	r := s.db.WithContext(ctx).Model(&domain.Task{}).Where("id = ? AND lease_token = ?", t.ID, token).Select("*").Omit("id", "created_at").Updates(t)
	if r.Error != nil {
		// A uniqueness conflict (e.g. another code has reserved this account)
		// must not retain an otherwise idle lease or persist partial confirmation.
		_ = s.db.WithContext(ctx).Model(&domain.Task{}).Where("id = ? AND lease_token = ?", t.ID, token).Updates(map[string]interface{}{"lease_token": "", "lease_until": time.Time{}}).Error
		return r.Error
	}
	if r.RowsAffected != 1 {
		return contract.ErrBusy
	}
	return nil
}
func (s *Store) Checkpoint(ctx context.Context, t *domain.Task) error {
	r := s.db.WithContext(ctx).Model(&domain.Task{}).Where("id = ? AND lease_token = ?", t.ID, t.LeaseToken).Select("*").Omit("id", "created_at").Updates(t)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return contract.ErrBusy
	}
	return nil
}

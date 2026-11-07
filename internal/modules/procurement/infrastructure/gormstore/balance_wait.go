package gormstore

import (
	"context"
	"errors"
	"time"

	"github.com/dujiao-next/internal/modules/procurement/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) CreateBalanceWait(ctx context.Context, wait *domain.BalanceWait) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "order_id"}}, DoNothing: true}).Create(wait).Error
}

func (s *Store) ListBalanceWaits(ctx context.Context, after uint, limit int, now time.Time) ([]domain.BalanceWait, error) {
	var rows []domain.BalanceWait
	err := s.db.WithContext(ctx).Where("id > ? AND status IN ?", after, []string{domain.BalanceWaitWaiting, domain.BalanceWaitReady}).
		Where("lease_until IS NULL OR lease_until <= ?", now).
		Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
		Order("id ASC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (s *Store) SetBalanceWaitReady(ctx context.Context, id uint, ready bool, now time.Time) error {
	updates := map[string]interface{}{"status": domain.BalanceWaitWaiting, "recovered_at": nil}
	if ready {
		updates["status"], updates["recovered_at"] = domain.BalanceWaitReady, now
	}
	return s.db.WithContext(ctx).Model(&domain.BalanceWait{}).
		Where("id = ? AND status IN ?", id, []string{domain.BalanceWaitWaiting, domain.BalanceWaitReady}).
		Where("lease_until IS NULL OR lease_until <= ?", now).Updates(updates).Error
}

func (s *Store) CloseBalanceWait(ctx context.Context, id uint, reason string) error {
	return s.db.WithContext(ctx).Model(&domain.BalanceWait{}).
		Where("id = ? AND status IN ?", id, []string{domain.BalanceWaitWaiting, domain.BalanceWaitReady}).
		Updates(map[string]interface{}{"status": domain.BalanceWaitClosed, "last_error": reason, "lease_token": "", "lease_until": nil}).Error
}

// The lease outlives the bounded SMTP exchange; a crashed worker can be retried.
func (s *Store) ClaimBalanceMail(ctx context.Context, id uint, token string, now time.Time) (*domain.BalanceWait, error) {
	result := s.db.WithContext(ctx).Model(&domain.BalanceWait{}).
		Where("id = ? AND status = ?", id, domain.BalanceWaitReady).
		Where("lease_until IS NULL OR lease_until <= ?", now).
		Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
		Updates(map[string]interface{}{"lease_token": token, "lease_until": now.Add(2 * time.Minute), "attempts": gorm.Expr("attempts + 1")})
	if result.Error != nil || result.RowsAffected == 0 {
		return nil, result.Error
	}
	var wait domain.BalanceWait
	err := s.db.WithContext(ctx).Where("id = ? AND lease_token = ?", id, token).First(&wait).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &wait, err
}

func (s *Store) FinishBalanceMail(ctx context.Context, id uint, token string, now time.Time, success bool) error {
	updates := map[string]interface{}{"lease_token": "", "lease_until": nil}
	if success {
		updates["status"], updates["sent_at"], updates["last_error"], updates["next_attempt_at"] = domain.BalanceWaitSent, now, "", nil
	} else {
		// A fresh balance check is required before every later delivery attempt.
		updates["status"], updates["recovered_at"] = domain.BalanceWaitWaiting, nil
		updates["next_attempt_at"], updates["last_error"] = now.Add(time.Minute), "email_send_failed"
	}
	return s.db.WithContext(ctx).Model(&domain.BalanceWait{}).
		Where("id = ? AND status = ? AND lease_token = ?", id, domain.BalanceWaitReady, token).Updates(updates).Error
}

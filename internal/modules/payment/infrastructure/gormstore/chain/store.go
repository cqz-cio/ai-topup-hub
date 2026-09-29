package chainstore

import (
	"context"
	"errors"
	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) *Store { return &Store{db: db} }
func mapError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return chain.ErrNotFound
	}
	return err
}
func (s *Store) Invoice(ctx context.Context, id uint) (*domain.ChainInvoice, error) {
	var v domain.ChainInvoice
	err := s.db.WithContext(ctx).Where("payment_id = ?", id).First(&v).Error
	return &v, mapError(err)
}
func (s *Store) PayableUntil(ctx context.Context, id uint) (*time.Time, error) {
	var order orderdomain.Order
	if err := s.db.WithContext(ctx).Select("id", "status", "expires_at").First(&order, id).Error; err != nil {
		return nil, mapError(err)
	}
	if order.Status != constants.OrderStatusPendingPayment {
		return nil, chain.ErrNotFound
	}
	return order.ExpiresAt, nil
}
func (s *Store) CreateInvoice(ctx context.Context, invoice *domain.ChainInvoice, cursor *domain.ChainCursor) (created bool, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(cursor).Error; err != nil {
			return err
		}
		r := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(invoice)
		created = r.RowsAffected > 0
		return r.Error
	})
	return
}
func (s *Store) Cursor(ctx context.Context, stream string) (*domain.ChainCursor, error) {
	var v domain.ChainCursor
	err := s.db.WithContext(ctx).First(&v, "stream = ?", stream).Error
	return &v, mapError(err)
}
func (s *Store) ApplyScan(ctx context.Context, cursor domain.ChainCursor, end uint64, hash string, transfers []domain.ChainTransfer) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := tx.Model(&domain.ChainCursor{}).Where("stream = ? AND next_block = ? AND previous_hash = ?", cursor.Stream, cursor.NextBlock, cursor.PreviousHash).Updates(map[string]interface{}{"next_block": end + 1, "previous_hash": hash, "updated_at": time.Now()})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return nil
		}
		for i := range transfers {
			v := &transfers[i]
			insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(v)
			if insert.Error != nil {
				return insert.Error
			}
			if insert.RowsAffected == 0 {
				continue
			}
			var invoice domain.ChainInvoice
			if err := tx.Where("amount_key = ?", cursor.Stream+"/"+v.Units).First(&invoice).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			v.PaymentID = invoice.PaymentID
			if v.Block < invoice.StartBlock {
				v.Reason = "before_invoice"
			} else if v.PaidAt.After(invoice.ExpiresAt) {
				v.Reason = "paid_after_expiry"
			} else if invoice.State != "pending" {
				v.Reason = "duplicate_payment"
			} else {
				r := tx.Model(&domain.ChainInvoice{}).Where("id = ? AND state = ?", invoice.ID, "pending").Updates(map[string]interface{}{"state": "confirmed", "transfer_key": v.Key})
				if r.Error != nil {
					return r.Error
				}
				if r.RowsAffected == 1 {
					v.State, v.Reason = "ready", ""
				}
			}
			if err := tx.Save(v).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Store) ReadyTransfers(ctx context.Context, stream string) ([]domain.ChainTransfer, error) {
	var rows []domain.ChainTransfer
	err := s.db.WithContext(ctx).Where("stream = ? AND state = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", stream, "ready", time.Now()).Order("block, log_index").Limit(30).Find(&rows).Error
	return rows, err
}
func (s *Store) InvoiceForTransfer(ctx context.Context, v domain.ChainTransfer) (*domain.ChainInvoice, error) {
	var invoice domain.ChainInvoice
	err := s.db.WithContext(ctx).Where("payment_id = ? AND transfer_key = ?", v.PaymentID, v.Key).First(&invoice).Error
	return &invoice, mapError(err)
}
func (s *Store) RetryTransfer(ctx context.Context, key string) error {
	return s.db.WithContext(ctx).Model(&domain.ChainTransfer{}).Where("key = ? AND state = ?", key, "ready").Update("next_attempt_at", time.Now().Add(time.Minute)).Error
}
func (s *Store) DeliverTransfer(ctx context.Context, key string) error {
	return s.db.WithContext(ctx).Model(&domain.ChainTransfer{}).Where("key = ? AND state = ?", key, "ready").Update("state", "delivered").Error
}
func (s *Store) Counts(ctx context.Context, stream string) (ready, review int64, err error) {
	err = s.db.WithContext(ctx).Model(&domain.ChainTransfer{}).Where("stream = ? AND state = ?", stream, "ready").Count(&ready).Error
	if err != nil {
		return
	}
	err = s.db.WithContext(ctx).Model(&domain.ChainTransfer{}).Where("stream = ? AND state = ?", stream, "manual_review").Count(&review).Error
	return
}
func (s *Store) Transfers(ctx context.Context, stream, state string, before uint64) ([]domain.ChainTransfer, error) {
	query := s.db.WithContext(ctx).Where("stream = ?", stream)
	if state != "" {
		query = query.Where("state = ?", state)
	}
	if before > 0 {
		query = query.Where("block < ?", before)
	}
	var rows []domain.ChainTransfer
	err := query.Order("block DESC, log_index DESC").Limit(100).Find(&rows).Error
	return rows, err
}

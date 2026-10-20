package chainstore

import (
	"context"
	"errors"
	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
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

// RechargePayableUntil uses the persisted recharge/payment pair. A recharge has
// no product order, and must never borrow another order's payment deadline.
func (s *Store) RechargePayableUntil(ctx context.Context, paymentID uint) (*time.Time, error) {
	var payment domain.Payment
	if err := s.db.WithContext(ctx).First(&payment, paymentID).Error; err != nil {
		return nil, mapError(err)
	}
	if payment.OrderID != 0 || (payment.Status != constants.PaymentStatusInitiated && payment.Status != constants.PaymentStatusPending) {
		return nil, chain.ErrNotFound
	}
	var recharge walletdomain.RechargeOrder
	if err := s.db.WithContext(ctx).Where("payment_id = ? AND status = ?", paymentID, constants.WalletRechargeStatusPending).First(&recharge).Error; err != nil {
		return nil, mapError(err)
	}
	if recharge.UserID == 0 || recharge.ChannelID != payment.ChannelID || recharge.Currency != payment.Currency || !recharge.PayableAmount.Equal(payment.Amount.Decimal) {
		return nil, chain.ErrNotFound
	}
	return payment.ExpiredAt, nil
}
func (s *Store) CreateInvoice(ctx context.Context, invoice *domain.ChainInvoice, cursor *domain.ChainCursor) (created bool, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(cursor).Error; err != nil {
			return err
		}
		// Serialize reservations with the persisted stream row. The no-op write
		// locks across processes too; a Go mutex would not protect other workers.
		if err := tx.Model(&domain.ChainCursor{}).Where("stream = ?", cursor.Stream).
			UpdateColumn("next_block", gorm.Expr("next_block")).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if !invoice.ExpiresAt.After(now) {
			return errors.New("chain_invoice_expired")
		}
		var reservations []domain.ChainInvoice
		if err := tx.Select("expires_at").Where("stream = ? AND units = ?", invoice.Stream, invoice.Units).
			Find(&reservations).Error; err != nil {
			return err
		}
		// Compare parsed instants, not SQLite timestamp strings: legacy rows
		// may contain a different UTC offset. All states reserve until expiry.
		for _, reservation := range reservations {
			if reservation.ExpiresAt.After(now) {
				return nil
			}
		}
		invoice.CreatedAt = now
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
			// Match the chain timestamp, never the time this scanner happens to
			// see the transfer. Chain timestamps have whole-second precision.
			var candidates []domain.ChainInvoice
			if err := tx.Where("stream = ? AND units = ? AND start_block <= ?",
				cursor.Stream, v.Units, v.Block).
				Find(&candidates).Error; err != nil {
				return err
			}
			var matches []domain.ChainInvoice
			for _, candidate := range candidates {
				// Half-open [start,end) windows prevent a boundary second from
				// belonging to both invoices. Legacy rows require no rewrite.
				if v.PaidAt.Unix() >= candidate.CreatedAt.Unix() && v.PaidAt.Unix() < candidate.ExpiresAt.Unix() {
					matches = append(matches, candidate)
				}
			}
			if len(matches) > 1 {
				return errors.New("chain_payment_windows_overlap")
			}
			if len(matches) == 0 {
				v.State, v.Reason = "manual_review", "outside_payment_window_or_amount"
				if err := tx.Save(v).Error; err != nil {
					return err
				}
				continue
			}
			invoice := matches[0]
			v.PaymentID = invoice.PaymentID
			if invoice.State != "pending" {
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

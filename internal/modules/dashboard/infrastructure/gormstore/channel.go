package gormstore

import (
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	"github.com/dujiao-next/internal/constants"
	dashboard "github.com/dujiao-next/internal/modules/dashboard/contract"
)

// GetTopChannels 获取支付渠道排行榜
func (r *Store) GetTopChannels(startAt, endAt time.Time, limit int) ([]dashboard.ChannelRankingRow, error) {
	if limit <= 0 {
		limit = 5
	}
	rows := make([]dashboard.ChannelRankingRow, 0)
	timeWhere, timeArgs := timeRangeQuery(r.db, "payments.created_at", startAt, endAt)
	args := append(timeArgs, constants.PaymentProviderWallet)
	if err := r.db.Model(&paymentdomain.Payment{}).
		Select(`
			payments.channel_id as channel_id,
			COALESCE(payment_channels.name, '') as channel_name,
			payments.provider_type as provider_type,
			payments.channel_type as channel_type,
			SUM(CASE WHEN payments.status = 'success' THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN payments.status = 'failed' THEN 1 ELSE 0 END) as failed_count,
			COALESCE(SUM(CASE WHEN payments.status = 'success' THEN payments.amount ELSE 0 END), 0) as success_amount
		`).
		Joins("LEFT JOIN payment_channels ON payment_channels.id = payments.channel_id").
		Where("payments.deleted_at IS NULL").
		Where(timeWhere+" AND payments.provider_type <> ?", args...).
		Group("payments.channel_id, payment_channels.name, payments.provider_type, payments.channel_type").
		Order("success_amount DESC, success_count DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

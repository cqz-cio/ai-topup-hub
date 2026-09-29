package gormstore

import (
	"fmt"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	dashboard "github.com/dujiao-next/internal/modules/dashboard/contract"
)

// GetOrderTrends 获取订单趋势
func (r *Store) GetOrderTrends(startAt, endAt time.Time) ([]dashboard.OrderTrendRow, error) {
	dayExpr := dateGroupExpr(r.db, "created_at", startAt.Location(), startAt)
	paidIn := quotedStatusList(paidOrderStatuses())

	rows := make([]dashboard.OrderTrendRow, 0)
	selectSQL := fmt.Sprintf(`
		%s as day,
		COUNT(*) as orders_total,
		COALESCE(SUM(CASE WHEN status IN (%s) THEN 1 ELSE 0 END), 0) as orders_paid
	`, dayExpr, paidIn)

	timeWhere, timeArgs := timeRangeQuery(r.db, "created_at", startAt, endAt)
	if err := r.db.Model(&orderdomain.Order{}).
		Select(selectSQL).
		Where("deleted_at IS NULL AND parent_id IS NULL AND "+timeWhere, timeArgs...).
		Group(dayExpr).
		Order("day ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetPaymentTrends 获取支付趋势
func (r *Store) GetPaymentTrends(startAt, endAt time.Time) ([]dashboard.PaymentTrendRow, error) {
	dayExpr := dateGroupExpr(r.db, "created_at", startAt.Location(), startAt)

	rows := make([]dashboard.PaymentTrendRow, 0)
	selectSQL := fmt.Sprintf(`
		%s as day,
		COALESCE(SUM(CASE WHEN status = '%s' THEN 1 ELSE 0 END), 0) as payments_success,
		COALESCE(SUM(CASE WHEN status = '%s' THEN 1 ELSE 0 END), 0) as payments_failed,
		COALESCE(SUM(CASE WHEN status = '%s' THEN amount ELSE 0 END), 0) as gmv_paid
	`, dayExpr, constants.PaymentStatusSuccess, constants.PaymentStatusFailed, constants.PaymentStatusSuccess)

	if err := onlinePaymentBase(r.db, startAt, endAt).
		Select(selectSQL).
		Group(dayExpr).
		Order("day ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

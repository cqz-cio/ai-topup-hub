package gormstore

import (
	"fmt"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	dashboard "github.com/dujiao-next/internal/modules/dashboard/contract"
)

// GetTopProducts 获取商品排行榜
func (r *Store) GetTopProducts(startAt, endAt time.Time, limit int) ([]dashboard.ProductRankingRow, error) {
	if limit <= 0 {
		limit = 5
	}
	rows := make([]dashboard.ProductRankingRow, 0)
	titleExpr := localizedJSONCoalesceExpr(r.db, "order_items.title_json")
	timeWhere, timeArgs := timeRangeQuery(r.db, "orders.created_at", startAt, endAt)
	args := append(timeArgs, paidOrderStatuses())
	if err := r.db.Model(&orderdomain.OrderItem{}).
		Select(fmt.Sprintf(`
			order_items.product_id as product_id,
			order_items.sku_id as sku_id,
			COALESCE(product_skus.sku_code, '') as sku_code,
			product_skus.spec_values_json as sku_spec_values_json,
			%s as title,
			COUNT(DISTINCT order_items.order_id) as paid_orders,
			COALESCE(SUM(order_items.quantity), 0) as quantity,
			COALESCE(SUM(order_items.total_price - order_items.coupon_discount), 0) as paid_amount,
			COALESCE(SUM(CASE WHEN order_items.cost_price > 0 THEN order_items.cost_price * order_items.quantity ELSE 0 END), 0) as total_cost
		`, titleExpr)).
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Joins("LEFT JOIN product_skus ON product_skus.id = order_items.sku_id AND product_skus.deleted_at IS NULL").
		Where("order_items.deleted_at IS NULL AND orders.deleted_at IS NULL AND "+timeWhere+" AND orders.status IN ?", args...).
		// 注意：不能把 product_skus.spec_values_json 直接放进 GROUP BY —— 在 Postgres 下 json 列没有等值运算符会报错。
		// 通过 GROUP BY 产品主键 product_skus.id，利用 PK 函数依赖让 Postgres 允许 SELECT sku_code/spec_values_json 不必聚合。
		Group("order_items.product_id, order_items.sku_id, product_skus.id, title").
		Order("paid_amount DESC, quantity DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

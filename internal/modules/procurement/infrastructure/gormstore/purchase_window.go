package gormstore

import (
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	"gorm.io/gorm"
)

func (s *Store) timeAfterSQL(column string) string {
	// SQLite stores timestamps as text; compare instants across UTC offsets.
	if s.db.Dialector.Name() == "sqlite" {
		return "julianday(" + column + ") > julianday(?)"
	}
	return column + " > ?"
}

// Filter before LIMIT so a backlog of excluded history cannot starve new work.
func (s *Store) ordersWithinPurchaseWindow(query *gorm.DB, window *procurementdomain.PurchaseWindow, requirePurchasable bool) *gorm.DB {
	if window == nil {
		return query
	}
	if window.PaidAfter.IsZero() {
		return query.Where("1 = 0")
	}
	parentStatus := ""
	if requirePurchasable {
		query = query.Where("o.status IN ('paid', 'fulfilling') AND COALESCE(o.refunded_amount, 0) = 0")
		parentStatus = "parent.status IN ('paid', 'fulfilling', 'partially_delivered', 'partially_refunded') AND "
	}
	return query.Where("o.id > ? AND o.deleted_at IS NULL", window.OrderIDFloor).
		Where(s.timeAfterSQL("o.paid_at"), window.PaidAfter).
		Where("o.parent_id IS NULL OR EXISTS (SELECT 1 FROM orders parent WHERE parent.id = o.parent_id AND parent.id > ? AND parent.deleted_at IS NULL AND "+parentStatus+s.timeAfterSQL("parent.paid_at")+")", window.OrderIDFloor, window.PaidAfter)
}

package gormutil

import (
	"errors"

	"github.com/dujiao-next/internal/constants"

	"gorm.io/gorm"
)

func ReserveManualStock(db *gorm.DB, model interface{}, id uint, quantity int) (int64, error) {
	if id == 0 || quantity <= 0 {
		return 0, errors.New("invalid manual stock reserve params")
	}
	result := db.Model(model).
		Where("id = ? AND deleted_at IS NULL AND manual_stock_total >= 0 AND manual_stock_total >= ?", id, quantity).
		Updates(map[string]interface{}{
			"manual_stock_total":  gorm.Expr("manual_stock_total - ?", quantity),
			"manual_stock_locked": gorm.Expr("manual_stock_locked + ?", quantity),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func ReleaseManualStock(db *gorm.DB, model interface{}, id uint, quantity int) (int64, error) {
	if id == 0 || quantity <= 0 {
		return 0, errors.New("invalid manual stock release params")
	}
	result := db.Model(model).
		Where("id = ? AND deleted_at IS NULL AND manual_stock_total >= 0 AND manual_stock_locked >= ?", id, quantity).
		Updates(map[string]interface{}{
			"manual_stock_total":  gorm.Expr("manual_stock_total + ?", quantity),
			"manual_stock_locked": gorm.Expr("manual_stock_locked - ?", quantity),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func ConsumeManualStock(db *gorm.DB, model interface{}, id uint, quantity int) (int64, error) {
	if id == 0 || quantity <= 0 {
		return 0, errors.New("invalid manual stock consume params")
	}
	result := db.Model(model).
		Where("id = ? AND deleted_at IS NULL AND manual_stock_total >= ? AND (manual_stock_locked >= ? OR (manual_stock_locked < ? AND manual_stock_total >= (? - manual_stock_locked)))",
			id, constants.ManualStockUnlimited+1, quantity, quantity, quantity).
		Updates(map[string]interface{}{
			"manual_stock_total":  gorm.Expr("manual_stock_total - CASE WHEN manual_stock_locked >= ? THEN 0 ELSE ? - manual_stock_locked END", quantity, quantity),
			"manual_stock_locked": gorm.Expr("CASE WHEN manual_stock_locked >= ? THEN manual_stock_locked - ? ELSE 0 END", quantity, quantity),
			"manual_stock_sold":   gorm.Expr("manual_stock_sold + ?", quantity),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

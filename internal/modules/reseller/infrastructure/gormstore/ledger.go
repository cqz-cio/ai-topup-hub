package gormstore

import (
	"errors"
	"strings"
	"time"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateLedgerEntryIfNotExists 按幂等键创建分销账务流水。
func (r *Store) CreateLedgerEntryIfNotExists(entry *resellerdomain.LedgerEntry) (bool, error) {
	if entry == nil {
		return false, errors.New("reseller ledger entry is nil")
	}
	key := strings.TrimSpace(entry.IdempotencyKey)
	if key == "" {
		return false, errors.New("reseller ledger idempotency key is empty")
	}
	existing, err := r.GetLedgerEntryByIdempotencyKey(key)
	if err != nil {
		return false, err
	}
	if existing != nil {
		return false, nil
	}
	now := time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now
	if err := r.db.Create(entry).Error; err != nil {
		var again resellerdomain.LedgerEntry
		if lookupErr := r.db.Where("idempotency_key = ? AND deleted_at IS NULL", key).First(&again).Error; lookupErr == nil {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetLedgerEntryByIdempotencyKey 按幂等键获取分销账务流水。
func (r *Store) GetLedgerEntryByIdempotencyKey(key string) (*resellerdomain.LedgerEntry, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	var row resellerdomain.LedgerEntry
	if err := r.db.Where("idempotency_key = ? AND deleted_at IS NULL", key).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// MarkDueLedgerEntriesAvailable 将到期确认流水转为可提现。
func (r *Store) MarkDueLedgerEntriesAvailable(now time.Time) (int64, error) {
	res := r.db.Model(&resellerdomain.LedgerEntry{}).
		Where("status = ? AND available_at IS NOT NULL AND available_at <= ? AND deleted_at IS NULL", resellerdomain.LedgerStatusPendingConfirm, now).
		Updates(map[string]interface{}{
			"status":     resellerdomain.LedgerStatusAvailable,
			"updated_at": now,
		})
	return res.RowsAffected, res.Error
}

// ListDueLedgerScopes 列出到期待确认流水涉及的分销商与币种组合。
func (r *Store) ListDueLedgerScopes(now time.Time) ([]resellercontract.LedgerScope, error) {
	scopes := make([]resellercontract.LedgerScope, 0)
	err := r.db.Model(&resellerdomain.LedgerEntry{}).
		Where("status = ? AND available_at IS NOT NULL AND available_at <= ? AND deleted_at IS NULL", resellerdomain.LedgerStatusPendingConfirm, now).
		Group("reseller_id, currency").
		Select("reseller_id, currency").
		Scan(&scopes).Error
	if err != nil {
		return nil, err
	}
	return scopes, nil
}

// ListLedgerEntries 分页列出分销账务流水。
func (r *Store) ListLedgerEntries(filter resellercontract.LedgerListFilter) ([]resellerdomain.LedgerEntry, int64, error) {
	rows := make([]resellerdomain.LedgerEntry, 0)
	query := r.db.Model(&resellerdomain.LedgerEntry{}).Where("deleted_at IS NULL")
	if filter.ResellerID != 0 {
		query = query.Where("reseller_id = ?", filter.ResellerID)
	}
	if currency := strings.TrimSpace(filter.Currency); currency != "" {
		query = query.Where("currency = ?", currency)
	}
	if typ := strings.TrimSpace(filter.Type); typ != "" {
		query = query.Where("type = ?", typ)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if filter.OrderID != 0 {
		query = query.Where("order_id = ?", filter.OrderID)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := applyPagination(query.Session(&gorm.Session{}), filter.Page, filter.PageSize).
		Order("id DESC").
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// SumLedgerAmount 汇总指定状态的分销账务金额。
func (r *Store) SumLedgerAmount(resellerID uint, currency string, statuses []string) (decimal.Decimal, error) {
	currency = strings.TrimSpace(currency)
	if resellerID == 0 || currency == "" || len(statuses) == 0 {
		return decimal.Zero, nil
	}
	var total decimal.Decimal
	err := r.db.Model(&resellerdomain.LedgerEntry{}).
		Where("reseller_id = ? AND currency = ? AND status IN ? AND deleted_at IS NULL", resellerID, currency, statuses).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	if err != nil {
		return decimal.Zero, err
	}
	return total.Round(2), nil
}

// SumLedgerAmountByOrderAndType 汇总指定订单、指定类型的流水金额（含正负号），用于退款扣减的累计上限保护。
func (r *Store) SumLedgerAmountByOrderAndType(orderID uint, ledgerType string) (decimal.Decimal, error) {
	ledgerType = strings.TrimSpace(ledgerType)
	if orderID == 0 || ledgerType == "" {
		return decimal.Zero, nil
	}
	var total decimal.Decimal
	err := r.db.Model(&resellerdomain.LedgerEntry{}).
		Where("order_id = ? AND type = ? AND deleted_at IS NULL", orderID, ledgerType).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	if err != nil {
		return decimal.Zero, err
	}
	return total.Round(2), nil
}

// SumLedgerAmountGroupedByStatus 一次性按状态分组汇总分销账务金额，避免逐状态多次查询。
func (r *Store) SumLedgerAmountGroupedByStatus(resellerID uint, currency string, statuses []string) (map[string]decimal.Decimal, error) {
	currency = strings.TrimSpace(currency)
	result := make(map[string]decimal.Decimal, len(statuses))
	if resellerID == 0 || currency == "" || len(statuses) == 0 {
		return result, nil
	}
	type sumRow struct {
		Status string
		Total  decimal.Decimal
	}
	var rows []sumRow
	err := r.db.Model(&resellerdomain.LedgerEntry{}).
		Where("reseller_id = ? AND currency = ? AND status IN ? AND deleted_at IS NULL", resellerID, currency, statuses).
		Group("status").
		Select("status, COALESCE(SUM(amount), 0) AS total").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.Status] = row.Total.Round(2)
	}
	return result, nil
}

// ListAvailableLedgerEntriesForUpdate 锁定指定币种可提现正向流水。
func (r *Store) ListAvailableLedgerEntriesForUpdate(resellerID uint, currency string) ([]resellerdomain.LedgerEntry, error) {
	rows := make([]resellerdomain.LedgerEntry, 0)
	currency = strings.TrimSpace(currency)
	if resellerID == 0 || currency == "" {
		return rows, nil
	}
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("reseller_id = ? AND currency = ? AND status = ? AND withdraw_request_id IS NULL AND amount > 0 AND deleted_at IS NULL",
			resellerID,
			currency,
			resellerdomain.LedgerStatusAvailable,
		).
		Order("available_at ASC, id ASC").
		Find(&rows).Error
	return rows, err
}

// UpdateLedgerEntry 更新单条分销账务流水。
func (r *Store) UpdateLedgerEntry(entry *resellerdomain.LedgerEntry) error {
	if entry == nil {
		return errors.New("reseller ledger entry is nil")
	}
	entry.UpdatedAt = time.Now()
	return r.db.Model(&resellerdomain.LedgerEntry{}).
		Where("id = ? AND deleted_at IS NULL", entry.ID).
		Select("*").
		Updates(entry).Error
}

// BatchUpdateLedgerEntries 批量更新分销账务流水。
func (r *Store) BatchUpdateLedgerEntries(ids []uint, updates map[string]interface{}) error {
	if len(ids) == 0 {
		return nil
	}
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["updated_at"] = time.Now()
	return r.db.Model(&resellerdomain.LedgerEntry{}).Where("id IN ? AND deleted_at IS NULL", ids).Updates(updates).Error
}

// BatchUpdateLedgerEntriesByWithdrawID 按提现单 ID 批量更新分销账务流水。
func (r *Store) BatchUpdateLedgerEntriesByWithdrawID(withdrawID uint, updates map[string]interface{}) error {
	if withdrawID == 0 {
		return nil
	}
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["updated_at"] = time.Now()
	return r.db.Model(&resellerdomain.LedgerEntry{}).
		Where("withdraw_request_id = ? AND deleted_at IS NULL", withdrawID).
		Updates(updates).Error
}

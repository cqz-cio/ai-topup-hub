package gormstore

import (
	"errors"
	"strings"
	"time"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetOrCreateBalanceAccountForUpdate 获取或创建并锁定同币种余额账户。
func (r *Store) GetOrCreateBalanceAccountForUpdate(resellerID uint, currency string) (*resellerdomain.BalanceAccount, error) {
	currency = strings.TrimSpace(currency)
	if resellerID == 0 || currency == "" {
		return nil, errors.New("invalid reseller balance account")
	}
	var row resellerdomain.BalanceAccount
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("reseller_id = ? AND currency = ? AND deleted_at IS NULL", resellerID, currency).
		First(&row).Error
	if err == nil {
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := time.Now()
	row = resellerdomain.BalanceAccount{
		ResellerID: resellerID,
		Currency:   currency,
		Status:     resellerdomain.BalanceStatusNormal,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := r.db.Create(&row).Error; err != nil {
		return nil, err
	}
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND deleted_at IS NULL", row.ID).
		First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// ListBalanceAccounts 分页列出分销商余额账户。
func (r *Store) ListBalanceAccounts(filter resellercontract.BalanceAccountListFilter) ([]resellerdomain.BalanceAccount, int64, error) {
	rows := make([]resellerdomain.BalanceAccount, 0)
	query := r.db.Model(&resellerdomain.BalanceAccount{}).Where("deleted_at IS NULL")
	if filter.ResellerID != 0 {
		query = query.Where("reseller_id = ?", filter.ResellerID)
	}
	if currency := strings.TrimSpace(filter.Currency); currency != "" {
		query = query.Where("currency = ?", currency)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := applyPagination(query.Session(&gorm.Session{}), filter.Page, filter.PageSize).
		Order("currency ASC, id DESC").
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// UpdateBalanceAccount 更新分销余额账户缓存。
func (r *Store) UpdateBalanceAccount(account *resellerdomain.BalanceAccount) error {
	if account == nil {
		return errors.New("reseller balance account is nil")
	}
	account.UpdatedAt = time.Now()
	return r.db.Model(&resellerdomain.BalanceAccount{}).
		Where("id = ? AND deleted_at IS NULL", account.ID).
		Select("*").
		Updates(account).Error
}

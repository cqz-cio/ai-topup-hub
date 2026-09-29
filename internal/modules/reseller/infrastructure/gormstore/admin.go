package gormstore

import (
	"strconv"
	"strings"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"gorm.io/gorm"
)

// ListAdminResellerLedgerEntries 分页列出管理端分销账务流水。
func (r *Store) ListAdminResellerLedgerEntries(filter resellercontract.AdminLedgerListFilter) ([]resellerdomain.LedgerEntry, int64, error) {
	rows := make([]resellerdomain.LedgerEntry, 0)
	query := r.db.Model(&resellerdomain.LedgerEntry{}).
		Preload("Profile", "deleted_at IS NULL").
		Preload("Profile.User", "deleted_at IS NULL").
		Preload("Order", "deleted_at IS NULL").
		Where("reseller_ledger_entries.deleted_at IS NULL")

	query = r.applyAdminResellerProfileFilters(query, "reseller_ledger_entries", filter.ResellerID, filter.UserID, filter.Keyword, "")
	if currency := strings.TrimSpace(filter.Currency); currency != "" {
		query = query.Where("reseller_ledger_entries.currency = ?", currency)
	}
	if typ := strings.TrimSpace(filter.Type); typ != "" {
		query = query.Where("reseller_ledger_entries.type = ?", typ)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("reseller_ledger_entries.status = ?", status)
	}
	if filter.OrderID != 0 {
		query = query.Where("reseller_ledger_entries.order_id = ?", filter.OrderID)
	}
	if orderNo := strings.TrimSpace(filter.OrderNo); orderNo != "" {
		query = query.Joins("LEFT JOIN orders o_filter ON o_filter.id = reseller_ledger_entries.order_id AND o_filter.deleted_at IS NULL").
			Where("o_filter.order_no = ?", orderNo)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("reseller_ledger_entries.created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("reseller_ledger_entries.created_at <= ?", *filter.CreatedTo)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := applyPagination(query.Session(&gorm.Session{}), filter.Page, filter.PageSize).
		Order("reseller_ledger_entries.id DESC").
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ListAdminResellerBalanceAccounts 分页列出管理端分销余额账户。
func (r *Store) ListAdminResellerBalanceAccounts(filter resellercontract.AdminBalanceAccountListFilter) ([]resellerdomain.BalanceAccount, int64, error) {
	rows := make([]resellerdomain.BalanceAccount, 0)
	query := r.db.Model(&resellerdomain.BalanceAccount{}).
		Preload("Profile", "deleted_at IS NULL").
		Preload("Profile.User", "deleted_at IS NULL").
		Where("reseller_balance_accounts.deleted_at IS NULL")

	query = r.applyAdminResellerProfileFilters(query, "reseller_balance_accounts", filter.ResellerID, filter.UserID, filter.Keyword, "")
	if currency := strings.TrimSpace(filter.Currency); currency != "" {
		query = query.Where("reseller_balance_accounts.currency = ?", currency)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("reseller_balance_accounts.status = ?", status)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := applyPagination(query.Session(&gorm.Session{}), filter.Page, filter.PageSize).
		Order("reseller_balance_accounts.id DESC").
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ListAdminResellerWithdrawRequests 分页列出管理端分销提现申请。
func (r *Store) ListAdminResellerWithdrawRequests(filter resellercontract.AdminWithdrawListFilter) ([]resellerdomain.WithdrawRequest, int64, error) {
	rows := make([]resellerdomain.WithdrawRequest, 0)
	query := r.db.Model(&resellerdomain.WithdrawRequest{}).
		Preload("Profile", "deleted_at IS NULL").
		Preload("Profile.User", "deleted_at IS NULL").
		Preload("Processor", "deleted_at IS NULL").
		Where("reseller_withdraw_requests.deleted_at IS NULL")

	query = r.applyAdminResellerProfileFilters(query, "reseller_withdraw_requests", filter.ResellerID, filter.UserID, filter.Keyword, "reseller_withdraw_requests.account")
	if currency := strings.TrimSpace(filter.Currency); currency != "" {
		query = query.Where("reseller_withdraw_requests.currency = ?", currency)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("reseller_withdraw_requests.status = ?", status)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("reseller_withdraw_requests.created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("reseller_withdraw_requests.created_at <= ?", *filter.CreatedTo)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := applyPagination(query.Session(&gorm.Session{}), filter.Page, filter.PageSize).
		Order("reseller_withdraw_requests.id DESC").
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *Store) applyAdminResellerProfileFilters(query *gorm.DB, table string, resellerID uint, userID uint, keyword string, accountColumn string) *gorm.DB {
	if resellerID != 0 {
		query = query.Where(table+".reseller_id = ?", resellerID)
	}
	keyword = strings.TrimSpace(keyword)
	if userID == 0 && keyword == "" {
		return query
	}

	query = query.
		Joins("LEFT JOIN reseller_profiles rp_filter ON rp_filter.id = " + table + ".reseller_id AND rp_filter.deleted_at IS NULL").
		Joins("LEFT JOIN users u_filter ON u_filter.id = rp_filter.user_id AND u_filter.deleted_at IS NULL")
	if userID != 0 {
		query = query.Where("rp_filter.user_id = ?", userID)
	}
	if keyword == "" {
		return query
	}

	like := "%" + keyword + "%"
	conditions := []string{"u_filter.email LIKE ?", "u_filter.display_name LIKE ?"}
	args := []interface{}{like, like}
	if id, err := strconv.ParseUint(keyword, 10, 64); err == nil && id > 0 {
		conditions = append(conditions, "rp_filter.id = ?")
		args = append(args, uint(id))
	}
	if accountColumn != "" {
		conditions = append(conditions, accountColumn+" LIKE ?")
		args = append(args, like)
	}
	return query.Where("("+strings.Join(conditions, " OR ")+")", args...)
}

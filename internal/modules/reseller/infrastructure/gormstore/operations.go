package gormstore

import (
	"database/sql"
	"sort"
	"strings"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/dujiao-next/internal/constants"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/shopspring/decimal"
)

func resellerOperationsPaidStatuses() []string {
	return []string{
		constants.OrderStatusPaid,
		constants.OrderStatusFulfilling,
		constants.OrderStatusPartiallyDelivered,
		constants.OrderStatusPartiallyRefunded,
		constants.OrderStatusDelivered,
		constants.OrderStatusCompleted,
	}
}

func (r *Store) GetOverview(startAt, endAt time.Time) (resellercontract.OperationsOverviewRow, error) {
	var out resellercontract.OperationsOverviewRow
	if r == nil || r.db == nil {
		return out, nil
	}
	if err := r.scanLifecycle(&out.Lifecycle); err != nil {
		return out, err
	}
	if err := r.scanOrderOverview(startAt, endAt, &out.Orders); err != nil {
		return out, err
	}
	top, err := r.scanTopResellers(startAt, endAt)
	if err != nil {
		return out, err
	}
	out.TopResellers = top
	return out, nil
}

func (r *Store) GetFinance(startAt, endAt time.Time) (resellercontract.OperationsFinanceRowSet, error) {
	out := resellercontract.OperationsFinanceRowSet{
		PeriodCurrencyRows:  []resellercontract.OperationsPeriodCurrencyRow{},
		CurrentCurrencyRows: []resellercontract.OperationsCurrentCurrencyRow{},
	}
	if r == nil || r.db == nil {
		return out, nil
	}
	period, err := r.scanPeriodCurrencyRows(startAt, endAt)
	if err != nil {
		return out, err
	}
	current, err := r.scanCurrentCurrencyRows()
	if err != nil {
		return out, err
	}
	out.PeriodCurrencyRows = period
	out.CurrentCurrencyRows = current
	return out, nil
}

func (r *Store) scanLifecycle(out *resellercontract.OperationsLifecycleRow) error {
	type profileScan struct {
		ProfilesTotal            int64
		ProfilesPendingReview    int64
		ProfilesActive           int64
		ProfilesRejected         int64
		ProfilesDisabled         int64
		ProfilesSettlementFrozen int64
	}
	var profiles profileScan
	if err := r.db.Model(&resellerdomain.Profile{}).
		Where("deleted_at IS NULL").
		Select(`
			COUNT(1) AS profiles_total,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS profiles_pending_review,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS profiles_active,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS profiles_rejected,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS profiles_disabled,
			SUM(CASE WHEN settlement_status = ? THEN 1 ELSE 0 END) AS profiles_settlement_frozen
		`,
			resellerdomain.ProfileStatusPendingReview,
			resellerdomain.ProfileStatusActive,
			resellerdomain.ProfileStatusRejected,
			resellerdomain.ProfileStatusDisabled,
			resellerdomain.SettlementStatusFrozen,
		).
		Scan(&profiles).Error; err != nil {
		return err
	}
	out.ProfilesTotal = profiles.ProfilesTotal
	out.ProfilesPendingReview = profiles.ProfilesPendingReview
	out.ProfilesActive = profiles.ProfilesActive
	out.ProfilesRejected = profiles.ProfilesRejected
	out.ProfilesDisabled = profiles.ProfilesDisabled
	out.ProfilesSettlementFrozen = profiles.ProfilesSettlementFrozen

	type domainScan struct {
		DomainsTotal               int64
		DomainsPendingReview       int64
		DomainsActive              int64
		DomainsDisabled            int64
		DomainsPendingVerification int64
		DomainsVerified            int64
		CustomDomains              int64
		Subdomains                 int64
	}
	var domains domainScan
	if err := r.db.Model(&resellerdomain.Domain{}).
		Where("deleted_at IS NULL").
		Select(`
			COUNT(1) AS domains_total,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS domains_pending_review,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS domains_active,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS domains_disabled,
			SUM(CASE WHEN verification_status = ? THEN 1 ELSE 0 END) AS domains_pending_verification,
			SUM(CASE WHEN verification_status = ? THEN 1 ELSE 0 END) AS domains_verified,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS custom_domains,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS subdomains
		`,
			resellerdomain.DomainStatusPendingReview,
			resellerdomain.DomainStatusActive,
			resellerdomain.DomainStatusDisabled,
			resellerdomain.DomainVerificationPending,
			resellerdomain.DomainVerificationVerified,
			resellerdomain.DomainTypeCustom,
			resellerdomain.DomainTypeSubdomain,
		).
		Scan(&domains).Error; err != nil {
		return err
	}
	out.DomainsTotal = domains.DomainsTotal
	out.DomainsPendingReview = domains.DomainsPendingReview
	out.DomainsActive = domains.DomainsActive
	out.DomainsDisabled = domains.DomainsDisabled
	out.DomainsPendingVerification = domains.DomainsPendingVerification
	out.DomainsVerified = domains.DomainsVerified
	out.CustomDomains = domains.CustomDomains
	out.Subdomains = domains.Subdomains

	if err := r.db.Model(&resellerdomain.SiteConfig{}).Where("deleted_at IS NULL").Count(&out.SiteConfigsTotal).Error; err != nil {
		return err
	}
	return r.db.Model(&resellerdomain.Profile{}).
		Joins("LEFT JOIN reseller_site_configs ON reseller_site_configs.reseller_id = reseller_profiles.id AND reseller_site_configs.deleted_at IS NULL").
		Where("reseller_profiles.status = ? AND reseller_profiles.deleted_at IS NULL", resellerdomain.ProfileStatusActive).
		Where("reseller_site_configs.id IS NULL").
		Count(&out.ActiveProfilesWithoutSiteConfig).Error
}

func (r *Store) scanOrderOverview(startAt, endAt time.Time, out *resellercontract.OperationsOrdersRow) error {
	paidStatuses := resellerOperationsPaidStatuses()
	type orderScan struct {
		OrdersTotal               int64
		PaidOrders                int64
		CompletedOrders           int64
		RefundedOrders            int64
		ActiveResellersWithOrders int64
	}
	var orders orderScan
	err := r.db.Model(&orderdomain.Order{}).
		Where("orders.deleted_at IS NULL AND orders.reseller_id IS NOT NULL AND orders.parent_id IS NULL AND orders.created_at >= ? AND orders.created_at < ?", startAt, endAt).
		Select(`
			COUNT(1) AS orders_total,
			SUM(CASE WHEN orders.status IN ? THEN 1 ELSE 0 END) AS paid_orders,
			SUM(CASE WHEN orders.status = ? THEN 1 ELSE 0 END) AS completed_orders,
			SUM(CASE WHEN orders.status = ? THEN 1 ELSE 0 END) AS refunded_orders,
			COUNT(DISTINCT CASE WHEN orders.status IN ? THEN orders.reseller_id ELSE NULL END) AS active_resellers_with_orders
		`, paidStatuses, constants.OrderStatusCompleted, constants.OrderStatusRefunded, paidStatuses).
		Scan(&orders).Error
	if err != nil {
		return err
	}
	out.OrdersTotal = orders.OrdersTotal
	out.PaidOrders = orders.PaidOrders
	out.CompletedOrders = orders.CompletedOrders
	out.RefundedOrders = orders.RefundedOrders
	out.ActiveResellersWithOrders = orders.ActiveResellersWithOrders

	return r.db.Model(&resellerdomain.OrderSnapshot{}).
		Where("profit_eligible = ? AND profit_block_reason IN ? AND created_at >= ? AND created_at < ? AND deleted_at IS NULL",
			false,
			[]string{"self_dealing_owner", "self_dealing_related_account"},
			startAt,
			endAt,
		).
		Count(&out.SelfDealingBlockedOrders).Error
}

func (r *Store) scanTopResellers(startAt, endAt time.Time) ([]resellercontract.OperationsTopResellerRow, error) {
	type topScan struct {
		ResellerID          uint
		UserID              uint
		Email               string
		DisplayName         string
		OrdersTotal         int64
		PaidOrders          int64
		ActiveDomains       int64
		SiteConfiguredCount int64
		LastOrderAt         sql.NullString
	}
	rows := []topScan{}
	err := r.db.Table("reseller_profiles").
		Select(`
			reseller_profiles.id AS reseller_id,
			reseller_profiles.user_id AS user_id,
			users.email AS email,
			users.display_name AS display_name,
			COUNT(orders.id) AS orders_total,
			SUM(CASE WHEN orders.status IN ? THEN 1 ELSE 0 END) AS paid_orders,
			COALESCE(domain_counts.active_domains, 0) AS active_domains,
			COUNT(DISTINCT reseller_site_configs.id) AS site_configured_count,
			MAX(orders.created_at) AS last_order_at
		`, resellerOperationsPaidStatuses()).
		Joins("JOIN orders ON orders.reseller_id = reseller_profiles.id AND orders.parent_id IS NULL AND orders.created_at >= ? AND orders.created_at < ? AND orders.deleted_at IS NULL", startAt, endAt).
		Joins("LEFT JOIN users ON users.id = reseller_profiles.user_id AND users.deleted_at IS NULL").
		Joins("LEFT JOIN (SELECT reseller_id, COUNT(1) AS active_domains FROM reseller_domains WHERE status = ? AND deleted_at IS NULL GROUP BY reseller_id) AS domain_counts ON domain_counts.reseller_id = reseller_profiles.id", resellerdomain.DomainStatusActive).
		Joins("LEFT JOIN reseller_site_configs ON reseller_site_configs.reseller_id = reseller_profiles.id AND reseller_site_configs.deleted_at IS NULL").
		Where("reseller_profiles.deleted_at IS NULL").
		Group("reseller_profiles.id, reseller_profiles.user_id, users.email, users.display_name, domain_counts.active_domains").
		Order("paid_orders DESC, orders_total DESC, reseller_profiles.id DESC").
		Limit(10).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]resellercontract.OperationsTopResellerRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, resellercontract.OperationsTopResellerRow{
			ResellerID:     row.ResellerID,
			UserID:         row.UserID,
			Email:          row.Email,
			DisplayName:    row.DisplayName,
			OrdersTotal:    row.OrdersTotal,
			PaidOrders:     row.PaidOrders,
			ActiveDomains:  row.ActiveDomains,
			SiteConfigured: row.SiteConfiguredCount > 0,
			LastOrderAt:    resellerOperationsParseDBTime(row.LastOrderAt),
		})
	}
	return out, nil
}

func (r *Store) scanPeriodCurrencyRows(startAt, endAt time.Time) ([]resellercontract.OperationsPeriodCurrencyRow, error) {
	rowsByCurrency := map[string]*resellercontract.OperationsPeriodCurrencyRow{}
	paidStatuses := resellerOperationsPaidStatuses()

	type orderCurrencyScan struct {
		Currency    string
		OrdersTotal int64
		PaidOrders  int64
		GMVPaid     decimal.Decimal
	}
	orderRows := []orderCurrencyScan{}
	if err := r.db.Model(&orderdomain.Order{}).
		Select(`
			currency,
			COUNT(1) AS orders_total,
			SUM(CASE WHEN status IN ? THEN 1 ELSE 0 END) AS paid_orders,
			COALESCE(SUM(CASE WHEN status IN ? THEN total_amount ELSE 0 END), 0) AS gmv_paid
		`, paidStatuses, paidStatuses).
		Where("deleted_at IS NULL AND reseller_id IS NOT NULL AND parent_id IS NULL AND created_at >= ? AND created_at < ?", startAt, endAt).
		Group("currency").
		Scan(&orderRows).Error; err != nil {
		return nil, err
	}
	for _, row := range orderRows {
		target := resellerOperationsPeriodCurrencyTarget(rowsByCurrency, row.Currency)
		target.OrdersTotal = row.OrdersTotal
		target.PaidOrders = row.PaidOrders
		target.GMVPaid = row.GMVPaid.Round(2)
	}

	type ledgerCurrencyScan struct {
		Currency       string
		ProfitEarned   decimal.Decimal
		RefundDeducted decimal.Decimal
	}
	ledgerRows := []ledgerCurrencyScan{}
	if err := r.db.Model(&resellerdomain.LedgerEntry{}).
		Select(`
			currency,
			COALESCE(SUM(CASE WHEN type = ? THEN amount ELSE 0 END), 0) AS profit_earned,
			ABS(COALESCE(SUM(CASE WHEN type = ? THEN amount ELSE 0 END), 0)) AS refund_deducted
		`, resellerdomain.LedgerTypeOrderProfit, resellerdomain.LedgerTypeRefundDeduct).
		Where("created_at >= ? AND created_at < ? AND status <> ? AND deleted_at IS NULL", startAt, endAt, resellerdomain.LedgerStatusCanceled).
		Group("currency").
		Scan(&ledgerRows).Error; err != nil {
		return nil, err
	}
	for _, row := range ledgerRows {
		target := resellerOperationsPeriodCurrencyTarget(rowsByCurrency, row.Currency)
		target.ProfitEarned = row.ProfitEarned.Round(2)
		target.RefundDeducted = row.RefundDeducted.Round(2)
	}

	type withdrawCurrencyScan struct {
		Currency     string
		WithdrawPaid decimal.Decimal
	}
	withdrawRows := []withdrawCurrencyScan{}
	if err := r.db.Model(&resellerdomain.WithdrawRequest{}).
		Select("currency, COALESCE(SUM(amount), 0) AS withdraw_paid").
		Where("status = ? AND processed_at IS NOT NULL AND processed_at >= ? AND processed_at < ? AND deleted_at IS NULL", resellerdomain.WithdrawStatusPaid, startAt, endAt).
		Group("currency").
		Scan(&withdrawRows).Error; err != nil {
		return nil, err
	}
	for _, row := range withdrawRows {
		target := resellerOperationsPeriodCurrencyTarget(rowsByCurrency, row.Currency)
		target.WithdrawPaid = row.WithdrawPaid.Round(2)
	}

	return resellerOperationsSortedPeriodRows(rowsByCurrency), nil
}

func (r *Store) scanCurrentCurrencyRows() ([]resellercontract.OperationsCurrentCurrencyRow, error) {
	rowsByCurrency := map[string]*resellercontract.OperationsCurrentCurrencyRow{}

	type balanceCurrencyScan struct {
		Currency                string
		AvailableBalance        decimal.Decimal
		LockedBalance           decimal.Decimal
		NegativeBalance         decimal.Decimal
		NegativeBalanceAccounts int64
		FrozenBalanceAccounts   int64
	}
	balanceRows := []balanceCurrencyScan{}
	if err := r.db.Model(&resellerdomain.BalanceAccount{}).
		Where("deleted_at IS NULL").
		Select(`
			currency,
			COALESCE(SUM(available_amount_cache), 0) AS available_balance,
			COALESCE(SUM(locked_amount_cache), 0) AS locked_balance,
			COALESCE(SUM(negative_amount_cache), 0) AS negative_balance,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS negative_balance_accounts,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS frozen_balance_accounts
		`, resellerdomain.BalanceStatusNegativeBalance, resellerdomain.BalanceStatusFrozenReview).
		Group("currency").
		Scan(&balanceRows).Error; err != nil {
		return nil, err
	}
	for _, row := range balanceRows {
		target := resellerOperationsCurrentCurrencyTarget(rowsByCurrency, row.Currency)
		target.AvailableBalance = row.AvailableBalance.Round(2)
		target.LockedBalance = row.LockedBalance.Round(2)
		target.NegativeBalance = row.NegativeBalance.Round(2)
		target.NegativeBalanceAccounts = row.NegativeBalanceAccounts
		target.FrozenBalanceAccounts = row.FrozenBalanceAccounts
	}

	type pendingWithdrawScan struct {
		Currency              string
		PendingWithdrawCount  int64
		PendingWithdrawAmount decimal.Decimal
	}
	withdrawRows := []pendingWithdrawScan{}
	if err := r.db.Model(&resellerdomain.WithdrawRequest{}).
		Select("currency, COUNT(1) AS pending_withdraw_count, COALESCE(SUM(amount), 0) AS pending_withdraw_amount").
		Where("status = ? AND deleted_at IS NULL", resellerdomain.WithdrawStatusPending).
		Group("currency").
		Scan(&withdrawRows).Error; err != nil {
		return nil, err
	}
	for _, row := range withdrawRows {
		target := resellerOperationsCurrentCurrencyTarget(rowsByCurrency, row.Currency)
		target.PendingWithdrawCount = row.PendingWithdrawCount
		target.PendingWithdrawAmount = row.PendingWithdrawAmount.Round(2)
	}

	return resellerOperationsSortedCurrentRows(rowsByCurrency), nil
}

func resellerOperationsPeriodCurrencyTarget(rows map[string]*resellercontract.OperationsPeriodCurrencyRow, currency string) *resellercontract.OperationsPeriodCurrencyRow {
	currency = resellerOperationsNormalizeCurrency(currency)
	if currency == "" {
		currency = "UNKNOWN"
	}
	if row, ok := rows[currency]; ok {
		return row
	}
	row := &resellercontract.OperationsPeriodCurrencyRow{
		Currency:       currency,
		GMVPaid:        decimal.Zero,
		ProfitEarned:   decimal.Zero,
		RefundDeducted: decimal.Zero,
		WithdrawPaid:   decimal.Zero,
	}
	rows[currency] = row
	return row
}

func resellerOperationsCurrentCurrencyTarget(rows map[string]*resellercontract.OperationsCurrentCurrencyRow, currency string) *resellercontract.OperationsCurrentCurrencyRow {
	currency = resellerOperationsNormalizeCurrency(currency)
	if currency == "" {
		currency = "UNKNOWN"
	}
	if row, ok := rows[currency]; ok {
		return row
	}
	row := &resellercontract.OperationsCurrentCurrencyRow{
		Currency:              currency,
		AvailableBalance:      decimal.Zero,
		LockedBalance:         decimal.Zero,
		NegativeBalance:       decimal.Zero,
		PendingWithdrawAmount: decimal.Zero,
	}
	rows[currency] = row
	return row
}

func resellerOperationsSortedPeriodRows(rows map[string]*resellercontract.OperationsPeriodCurrencyRow) []resellercontract.OperationsPeriodCurrencyRow {
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]resellercontract.OperationsPeriodCurrencyRow, 0, len(keys))
	for _, key := range keys {
		out = append(out, *rows[key])
	}
	return out
}

func resellerOperationsSortedCurrentRows(rows map[string]*resellercontract.OperationsCurrentCurrencyRow) []resellercontract.OperationsCurrentCurrencyRow {
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]resellercontract.OperationsCurrentCurrencyRow, 0, len(keys))
	for _, key := range keys {
		out = append(out, *rows[key])
	}
	return out
}

func resellerOperationsNormalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func resellerOperationsParseDBTime(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	raw := strings.TrimSpace(value.String)
	if raw == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed
		}
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05.999999999", raw, time.UTC); err == nil {
		return &parsed
	}
	return nil
}

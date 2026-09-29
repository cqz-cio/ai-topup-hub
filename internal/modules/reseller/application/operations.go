package application

import (
	"context"
	"strings"
	"time"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	reportingapp "github.com/dujiao-next/internal/modules/reporting/application"
	reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"
	"github.com/shopspring/decimal"
)

type OperationsService struct {
	store resellercontract.OperationsStore
}

func NewOperationsService(store resellercontract.OperationsStore) *OperationsService {
	return &OperationsService{store: store}
}

type OperationsOverviewResponse struct {
	Range        string                          `json:"range"`
	From         string                          `json:"from"`
	To           string                          `json:"to"`
	Timezone     string                          `json:"timezone"`
	Lifecycle    OperationsLifecycleResponse     `json:"lifecycle"`
	Orders       OperationsOrdersResponse        `json:"orders"`
	TopResellers []OperationsTopResellerResponse `json:"top_resellers"`
	Alerts       []OperationsAlertItem           `json:"alerts"`
}

type OperationsLifecycleResponse struct {
	ProfilesTotal                   int64 `json:"profiles_total"`
	ProfilesPendingReview           int64 `json:"profiles_pending_review"`
	ProfilesActive                  int64 `json:"profiles_active"`
	ProfilesRejected                int64 `json:"profiles_rejected"`
	ProfilesDisabled                int64 `json:"profiles_disabled"`
	ProfilesSettlementFrozen        int64 `json:"profiles_settlement_frozen"`
	DomainsTotal                    int64 `json:"domains_total"`
	DomainsPendingReview            int64 `json:"domains_pending_review"`
	DomainsActive                   int64 `json:"domains_active"`
	DomainsDisabled                 int64 `json:"domains_disabled"`
	DomainsPendingVerification      int64 `json:"domains_pending_verification"`
	DomainsVerified                 int64 `json:"domains_verified"`
	CustomDomains                   int64 `json:"custom_domains"`
	Subdomains                      int64 `json:"subdomains"`
	SiteConfigsTotal                int64 `json:"site_configs_total"`
	ActiveProfilesWithoutSiteConfig int64 `json:"active_profiles_without_site_config"`
}

type OperationsOrdersResponse struct {
	OrdersTotal                        int64  `json:"orders_total"`
	PaidOrders                         int64  `json:"paid_orders"`
	CompletedOrders                    int64  `json:"completed_orders"`
	RefundedOrders                     int64  `json:"refunded_orders"`
	SelfDealingBlockedOrders           int64  `json:"self_dealing_blocked_orders"`
	ActiveResellersWithOrders          int64  `json:"active_resellers_with_orders"`
	AveragePaidOrdersPerActiveReseller string `json:"average_paid_orders_per_active_reseller"`
}

type OperationsTopResellerResponse struct {
	ResellerID     uint   `json:"reseller_id"`
	UserID         uint   `json:"user_id"`
	Email          string `json:"email,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	OrdersTotal    int64  `json:"orders_total"`
	PaidOrders     int64  `json:"paid_orders"`
	ActiveDomains  int64  `json:"active_domains"`
	SiteConfigured bool   `json:"site_configured"`
	LastOrderAt    string `json:"last_order_at,omitempty"`
}

type OperationsAlertItem struct {
	Type  string `json:"type"`
	Level string `json:"level"`
	Value int64  `json:"value"`
}

type OperationsFinanceResponse struct {
	Range               string                              `json:"range"`
	From                string                              `json:"from"`
	To                  string                              `json:"to"`
	Timezone            string                              `json:"timezone"`
	PeriodCurrencyRows  []OperationsPeriodCurrencyResponse  `json:"period_currency_rows"`
	CurrentCurrencyRows []OperationsCurrentCurrencyResponse `json:"current_currency_rows"`
}

type OperationsPeriodCurrencyResponse struct {
	Currency       string `json:"currency"`
	OrdersTotal    int64  `json:"orders_total"`
	PaidOrders     int64  `json:"paid_orders"`
	GMVPaid        string `json:"gmv_paid"`
	ProfitEarned   string `json:"profit_earned"`
	RefundDeducted string `json:"refund_deducted"`
	WithdrawPaid   string `json:"withdraw_paid"`
}

type OperationsCurrentCurrencyResponse struct {
	Currency                string `json:"currency"`
	AvailableBalance        string `json:"available_balance"`
	LockedBalance           string `json:"locked_balance"`
	NegativeBalance         string `json:"negative_balance"`
	PendingWithdrawCount    int64  `json:"pending_withdraw_count"`
	PendingWithdrawAmount   string `json:"pending_withdraw_amount"`
	NegativeBalanceAccounts int64  `json:"negative_balance_accounts"`
	FrozenBalanceAccounts   int64  `json:"frozen_balance_accounts"`
}

func (s *OperationsService) GetOverview(_ context.Context, input reportingdomain.Query) (*OperationsOverviewResponse, error) {
	window, err := reportingapp.Resolve(input, time.Now())
	if err != nil {
		return nil, err
	}
	resp := emptyOperationsOverviewResponse(window)
	if s == nil || s.store == nil {
		return resp, nil
	}
	row, err := s.store.GetOverview(window.StartAt, window.EndAt)
	if err != nil {
		return nil, err
	}
	resp.Lifecycle = mapOperationsLifecycle(row.Lifecycle)
	resp.Orders = mapOperationsOrders(row.Orders)
	resp.TopResellers = mapOperationsTopResellers(row.TopResellers)
	resp.Alerts = buildOperationsAlerts(row)
	return resp, nil
}

func (s *OperationsService) GetFinance(_ context.Context, input reportingdomain.Query) (*OperationsFinanceResponse, error) {
	window, err := reportingapp.Resolve(input, time.Now())
	if err != nil {
		return nil, err
	}
	resp := emptyOperationsFinanceResponse(window)
	if s == nil || s.store == nil {
		return resp, nil
	}
	rows, err := s.store.GetFinance(window.StartAt, window.EndAt)
	if err != nil {
		return nil, err
	}
	resp.PeriodCurrencyRows = make([]OperationsPeriodCurrencyResponse, 0, len(rows.PeriodCurrencyRows))
	for _, row := range rows.PeriodCurrencyRows {
		resp.PeriodCurrencyRows = append(resp.PeriodCurrencyRows, OperationsPeriodCurrencyResponse{
			Currency:       normalizeOperationsCurrency(row.Currency),
			OrdersTotal:    row.OrdersTotal,
			PaidOrders:     row.PaidOrders,
			GMVPaid:        formatOperationsDecimal(row.GMVPaid),
			ProfitEarned:   formatOperationsDecimal(row.ProfitEarned),
			RefundDeducted: formatOperationsDecimal(row.RefundDeducted),
			WithdrawPaid:   formatOperationsDecimal(row.WithdrawPaid),
		})
	}
	resp.CurrentCurrencyRows = make([]OperationsCurrentCurrencyResponse, 0, len(rows.CurrentCurrencyRows))
	for _, row := range rows.CurrentCurrencyRows {
		resp.CurrentCurrencyRows = append(resp.CurrentCurrencyRows, OperationsCurrentCurrencyResponse{
			Currency:                normalizeOperationsCurrency(row.Currency),
			AvailableBalance:        formatOperationsDecimal(row.AvailableBalance),
			LockedBalance:           formatOperationsDecimal(row.LockedBalance),
			NegativeBalance:         formatOperationsDecimal(row.NegativeBalance),
			PendingWithdrawCount:    row.PendingWithdrawCount,
			PendingWithdrawAmount:   formatOperationsDecimal(row.PendingWithdrawAmount),
			NegativeBalanceAccounts: row.NegativeBalanceAccounts,
			FrozenBalanceAccounts:   row.FrozenBalanceAccounts,
		})
	}
	return resp, nil
}

func emptyOperationsOverviewResponse(window reportingdomain.Window) *OperationsOverviewResponse {
	return &OperationsOverviewResponse{
		Range:        window.Range,
		From:         window.StartAt.Format(time.RFC3339),
		To:           window.EndAt.Add(-time.Second).Format(time.RFC3339),
		Timezone:     window.Timezone,
		TopResellers: []OperationsTopResellerResponse{},
		Alerts:       []OperationsAlertItem{},
	}
}

func emptyOperationsFinanceResponse(window reportingdomain.Window) *OperationsFinanceResponse {
	return &OperationsFinanceResponse{
		Range:               window.Range,
		From:                window.StartAt.Format(time.RFC3339),
		To:                  window.EndAt.Add(-time.Second).Format(time.RFC3339),
		Timezone:            window.Timezone,
		PeriodCurrencyRows:  []OperationsPeriodCurrencyResponse{},
		CurrentCurrencyRows: []OperationsCurrentCurrencyResponse{},
	}
}

func mapOperationsLifecycle(row resellercontract.OperationsLifecycleRow) OperationsLifecycleResponse {
	return OperationsLifecycleResponse(row)
}

func mapOperationsOrders(row resellercontract.OperationsOrdersRow) OperationsOrdersResponse {
	average := decimal.Zero
	if row.ActiveResellersWithOrders > 0 {
		average = decimal.NewFromInt(row.PaidOrders).Div(decimal.NewFromInt(row.ActiveResellersWithOrders))
	}
	return OperationsOrdersResponse{
		OrdersTotal:                        row.OrdersTotal,
		PaidOrders:                         row.PaidOrders,
		CompletedOrders:                    row.CompletedOrders,
		RefundedOrders:                     row.RefundedOrders,
		SelfDealingBlockedOrders:           row.SelfDealingBlockedOrders,
		ActiveResellersWithOrders:          row.ActiveResellersWithOrders,
		AveragePaidOrdersPerActiveReseller: average.StringFixed(2),
	}
}

func mapOperationsTopResellers(rows []resellercontract.OperationsTopResellerRow) []OperationsTopResellerResponse {
	out := make([]OperationsTopResellerResponse, 0, len(rows))
	for _, row := range rows {
		item := OperationsTopResellerResponse{
			ResellerID:     row.ResellerID,
			UserID:         row.UserID,
			Email:          strings.TrimSpace(row.Email),
			DisplayName:    strings.TrimSpace(row.DisplayName),
			OrdersTotal:    row.OrdersTotal,
			PaidOrders:     row.PaidOrders,
			ActiveDomains:  row.ActiveDomains,
			SiteConfigured: row.SiteConfigured,
		}
		if row.LastOrderAt != nil {
			item.LastOrderAt = row.LastOrderAt.Format(time.RFC3339)
		}
		out = append(out, item)
	}
	return out
}

func buildOperationsAlerts(row resellercontract.OperationsOverviewRow) []OperationsAlertItem {
	alerts := make([]OperationsAlertItem, 0, 4)
	add := func(typ, level string, value int64) {
		if value > 0 {
			alerts = append(alerts, OperationsAlertItem{Type: typ, Level: level, Value: value})
		}
	}
	add("profiles_pending_review", "warning", row.Lifecycle.ProfilesPendingReview)
	add("domains_pending_review", "warning", row.Lifecycle.DomainsPendingReview)
	add("active_profiles_without_site_config", "info", row.Lifecycle.ActiveProfilesWithoutSiteConfig)
	add("self_dealing_blocked_orders", "warning", row.Orders.SelfDealingBlockedOrders)
	return alerts
}

func formatOperationsDecimal(value decimal.Decimal) string {
	return value.Round(2).StringFixed(2)
}

func normalizeOperationsCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

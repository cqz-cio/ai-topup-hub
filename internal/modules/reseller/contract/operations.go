package contract

import (
	"time"

	"github.com/shopspring/decimal"
)

// OperationsStore 是运营看板只读聚合端口。
type OperationsStore interface {
	GetOverview(startAt, endAt time.Time) (OperationsOverviewRow, error)
	GetFinance(startAt, endAt time.Time) (OperationsFinanceRowSet, error)
}

type OperationsLifecycleRow struct {
	ProfilesTotal                   int64
	ProfilesPendingReview           int64
	ProfilesActive                  int64
	ProfilesRejected                int64
	ProfilesDisabled                int64
	ProfilesSettlementFrozen        int64
	DomainsTotal                    int64
	DomainsPendingReview            int64
	DomainsActive                   int64
	DomainsDisabled                 int64
	DomainsPendingVerification      int64
	DomainsVerified                 int64
	CustomDomains                   int64
	Subdomains                      int64
	SiteConfigsTotal                int64
	ActiveProfilesWithoutSiteConfig int64
}

type OperationsOrdersRow struct {
	OrdersTotal               int64
	PaidOrders                int64
	CompletedOrders           int64
	RefundedOrders            int64
	SelfDealingBlockedOrders  int64
	ActiveResellersWithOrders int64
}

type OperationsTopResellerRow struct {
	ResellerID     uint
	UserID         uint
	Email          string
	DisplayName    string
	OrdersTotal    int64
	PaidOrders     int64
	ActiveDomains  int64
	SiteConfigured bool
	LastOrderAt    *time.Time
}

type OperationsOverviewRow struct {
	Lifecycle    OperationsLifecycleRow
	Orders       OperationsOrdersRow
	TopResellers []OperationsTopResellerRow
}

type OperationsPeriodCurrencyRow struct {
	Currency       string
	OrdersTotal    int64
	PaidOrders     int64
	GMVPaid        decimal.Decimal
	ProfitEarned   decimal.Decimal
	RefundDeducted decimal.Decimal
	WithdrawPaid   decimal.Decimal
}

type OperationsCurrentCurrencyRow struct {
	Currency                string
	AvailableBalance        decimal.Decimal
	LockedBalance           decimal.Decimal
	NegativeBalance         decimal.Decimal
	PendingWithdrawCount    int64
	PendingWithdrawAmount   decimal.Decimal
	NegativeBalanceAccounts int64
	FrozenBalanceAccounts   int64
}

type OperationsFinanceRowSet struct {
	PeriodCurrencyRows  []OperationsPeriodCurrencyRow
	CurrentCurrencyRows []OperationsCurrentCurrencyRow
}

package gormstore

import (
	"testing"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func seedResellerOperationsOrder(t *testing.T, db *gorm.DB, profile resellerdomain.Profile, orderNo string, status string, amount string, createdAt time.Time) orderdomain.Order {
	t.Helper()
	order := orderdomain.Order{
		OrderNo:              orderNo,
		Status:               status,
		Currency:             "USD",
		TotalAmount:          money.FromDecimal(decimal.RequireFromString(amount)),
		ResellerProfitAmount: money.FromDecimal(decimal.RequireFromString("0")),
		ResellerID:           &profile.ID,
		CreatedAt:            createdAt,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create reseller operations order failed: %v", err)
	}
	return order
}

func TestResellerOperationsRepositoryOverviewAggregatesLifecycleAndOrders(t *testing.T) {
	db := openResellerAccountingRepoTestDB(t)
	repo := New(db)
	active := seedResellerAccountingProfileWithEmail(t, db, "active-ops@example.test")
	pending := seedResellerAccountingProfileWithEmail(t, db, "pending-ops@example.test")
	pending.Status = resellerdomain.ProfileStatusPendingReview
	if err := db.Save(&pending).Error; err != nil {
		t.Fatalf("save pending profile failed: %v", err)
	}
	frozen := seedResellerAccountingProfileWithEmail(t, db, "frozen-ops@example.test")
	frozen.SettlementStatus = resellerdomain.SettlementStatusFrozen
	if err := db.Save(&frozen).Error; err != nil {
		t.Fatalf("save frozen profile failed: %v", err)
	}
	now := time.Now().UTC()
	seedResellerOperationsOrder(t, db, active, "ROPS-PAID-1", constants.OrderStatusPaid, "100.00", now.Add(-time.Hour))
	seedResellerOperationsOrder(t, db, active, "ROPS-PENDING-1", constants.OrderStatusPendingPayment, "80.00", now.Add(-time.Hour))
	seedResellerOperationsOrder(t, db, frozen, "ROPS-COMPLETE-1", constants.OrderStatusCompleted, "90.00", now.Add(-time.Hour))
	if err := db.Create(&resellerdomain.Domain{
		ResellerID:         active.ID,
		Domain:             "ops.example.test",
		Type:               resellerdomain.DomainTypeCustom,
		VerificationStatus: resellerdomain.DomainVerificationVerified,
		Status:             resellerdomain.DomainStatusActive,
	}).Error; err != nil {
		t.Fatalf("create domain failed: %v", err)
	}
	if err := db.Create(&resellerdomain.SiteConfig{ResellerID: active.ID, SiteName: "Active Ops"}).Error; err != nil {
		t.Fatalf("create site config failed: %v", err)
	}
	snapshot := resellerdomain.OrderSnapshot{
		OrderID:           9901,
		ResellerID:        active.ID,
		Currency:          "USD",
		ProfitEligible:    false,
		ProfitBlockReason: "self_dealing_owner",
		CreatedAt:         now.Add(-time.Hour),
	}
	if err := db.Create(&snapshot).Error; err != nil {
		t.Fatalf("create self dealing snapshot failed: %v", err)
	}
	if err := db.Model(&resellerdomain.OrderSnapshot{}).Where("id = ?", snapshot.ID).Update("profit_eligible", false).Error; err != nil {
		t.Fatalf("force profit eligible false failed: %v", err)
	}

	row, err := repo.GetOverview(now.Add(-24*time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetOverview failed: %v", err)
	}
	if row.Lifecycle.ProfilesTotal != 3 || row.Lifecycle.ProfilesPendingReview != 1 || row.Lifecycle.ProfilesSettlementFrozen != 1 {
		t.Fatalf("unexpected lifecycle row: %+v", row.Lifecycle)
	}
	if row.Lifecycle.DomainsActive != 1 || row.Lifecycle.SiteConfigsTotal != 1 || row.Lifecycle.ActiveProfilesWithoutSiteConfig != 1 {
		t.Fatalf("unexpected domain/site config row: %+v", row.Lifecycle)
	}
	if row.Orders.OrdersTotal != 3 || row.Orders.PaidOrders != 2 || row.Orders.SelfDealingBlockedOrders != 1 {
		t.Fatalf("unexpected orders row: %+v", row.Orders)
	}
}

func TestResellerOperationsRepositoryFinanceSplitsPeriodAndCurrentCurrencyRows(t *testing.T) {
	db := openResellerAccountingRepoTestDB(t)
	repo := New(db)
	profile := seedResellerAccountingProfileWithEmail(t, db, "finance-ops@example.test")
	now := time.Now().UTC()
	seedResellerOperationsOrder(t, db, profile, "ROPS-FIN-PAID", constants.OrderStatusPaid, "120.00", now.Add(-time.Hour))
	if err := db.Create(&resellerdomain.LedgerEntry{
		ResellerID:     profile.ID,
		Type:           resellerdomain.LedgerTypeOrderProfit,
		Amount:         money.FromDecimal(decimal.RequireFromString("30.00")),
		Currency:       "USD",
		IdempotencyKey: "ops-profit-1",
		Status:         resellerdomain.LedgerStatusAvailable,
		CreatedAt:      now.Add(-time.Hour),
	}).Error; err != nil {
		t.Fatalf("create profit ledger failed: %v", err)
	}
	if err := db.Create(&resellerdomain.LedgerEntry{
		ResellerID:     profile.ID,
		Type:           resellerdomain.LedgerTypeRefundDeduct,
		Amount:         money.FromDecimal(decimal.RequireFromString("-4.00")),
		Currency:       "USD",
		IdempotencyKey: "ops-refund-1",
		Status:         resellerdomain.LedgerStatusAvailable,
		CreatedAt:      now.Add(-time.Hour),
	}).Error; err != nil {
		t.Fatalf("create refund ledger failed: %v", err)
	}
	if err := db.Create(&resellerdomain.BalanceAccount{
		ResellerID:           profile.ID,
		Currency:             "USD",
		Status:               resellerdomain.BalanceStatusNormal,
		AvailableAmountCache: money.FromDecimal(decimal.RequireFromString("26.00")),
		LockedAmountCache:    money.FromDecimal(decimal.RequireFromString("8.00")),
		NegativeAmountCache:  money.FromDecimal(decimal.Zero),
		LastLedgerEntryID:    0,
	}).Error; err != nil {
		t.Fatalf("create balance failed: %v", err)
	}
	if err := db.Create(&resellerdomain.WithdrawRequest{
		ResellerID: profile.ID,
		Amount:     money.FromDecimal(decimal.RequireFromString("8.00")),
		Currency:   "USD",
		Channel:    "USDT",
		Account:    "Tops",
		Status:     resellerdomain.WithdrawStatusPending,
	}).Error; err != nil {
		t.Fatalf("create withdraw failed: %v", err)
	}

	rows, err := repo.GetFinance(now.Add(-24*time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetFinance failed: %v", err)
	}
	if len(rows.PeriodCurrencyRows) != 1 || rows.PeriodCurrencyRows[0].Currency != "USD" {
		t.Fatalf("unexpected period rows: %+v", rows.PeriodCurrencyRows)
	}
	period := rows.PeriodCurrencyRows[0]
	if !period.GMVPaid.Equal(decimal.RequireFromString("120.00")) || !period.ProfitEarned.Equal(decimal.RequireFromString("30.00")) || !period.RefundDeducted.Equal(decimal.RequireFromString("4.00")) {
		t.Fatalf("unexpected period money row: %+v", period)
	}
	current := rows.CurrentCurrencyRows[0]
	if current.PendingWithdrawCount != 1 || !current.PendingWithdrawAmount.Equal(decimal.RequireFromString("8.00")) || !current.AvailableBalance.Equal(decimal.RequireFromString("26.00")) {
		t.Fatalf("unexpected current money row: %+v", current)
	}
}

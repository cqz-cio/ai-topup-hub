package gormstore

import (
	"fmt"
	"testing"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func openResellerAccountingRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:reseller_accounting_repo_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&admindomain.Admin{},
		&userdomain.User{},
		&orderdomain.Order{},
		&paymentdomain.Payment{},
		&resellerdomain.Profile{},
		&resellerdomain.Domain{},
		&resellerdomain.SiteConfig{},
		&resellerdomain.OrderSnapshot{},
		&resellerdomain.LedgerEntry{},
		&resellerdomain.WithdrawRequest{},
		&resellerdomain.BalanceAccount{},
	); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return db
}

func seedResellerAccountingProfile(t *testing.T, db *gorm.DB) resellerdomain.Profile {
	t.Helper()
	user := userdomain.User{Email: fmt.Sprintf("reseller-%d@example.test", time.Now().UnixNano()), PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	profile := resellerdomain.Profile{
		UserID:           user.ID,
		Status:           resellerdomain.ProfileStatusActive,
		SettlementStatus: resellerdomain.SettlementStatusNormal,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create reseller profile failed: %v", err)
	}
	return profile
}

func seedResellerAccountingProfileWithEmail(t *testing.T, db *gorm.DB, email string) resellerdomain.Profile {
	t.Helper()
	user := userdomain.User{Email: email, PasswordHash: "hash", Status: constants.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	profile := resellerdomain.Profile{
		UserID:           user.ID,
		Status:           resellerdomain.ProfileStatusActive,
		SettlementStatus: resellerdomain.SettlementStatusNormal,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create reseller profile failed: %v", err)
	}
	return profile
}

func seedResellerAccountingOrder(t *testing.T, db *gorm.DB, orderNo string) orderdomain.Order {
	t.Helper()
	order := orderdomain.Order{
		OrderNo:     orderNo,
		Status:      constants.OrderStatusPaid,
		TotalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		Currency:    "USD",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	return order
}

func TestResellerAccountingRepositoryLedgerIdempotency(t *testing.T) {
	db := openResellerAccountingRepoTestDB(t)
	profile := seedResellerAccountingProfile(t, db)
	repo := New(db)
	orderID := uint(100)
	entry := &resellerdomain.LedgerEntry{
		ResellerID:     profile.ID,
		OrderID:        &orderID,
		Type:           resellerdomain.LedgerTypeOrderProfit,
		Amount:         money.FromDecimal(decimal.RequireFromString("12.34")),
		Currency:       "USD",
		IdempotencyKey: "order_profit:100",
		Status:         resellerdomain.LedgerStatusPendingConfirm,
	}
	created, err := repo.CreateLedgerEntryIfNotExists(entry)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	if !created {
		t.Fatal("first create should report created=true")
	}
	created, err = repo.CreateLedgerEntryIfNotExists(entry)
	if err != nil {
		t.Fatalf("second create failed: %v", err)
	}
	if created {
		t.Fatal("second create should report created=false")
	}
	var count int64
	if err := db.Model(&resellerdomain.LedgerEntry{}).Where("idempotency_key = ?", "order_profit:100").Count(&count).Error; err != nil {
		t.Fatalf("count ledger failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one ledger row, got %d", count)
	}
}

func TestResellerAccountingRepositoryMarkDueLedgersAvailable(t *testing.T) {
	db := openResellerAccountingRepoTestDB(t)
	profile := seedResellerAccountingProfile(t, db)
	repo := New(db)
	now := time.Now()
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)
	rows := []resellerdomain.LedgerEntry{
		{ResellerID: profile.ID, Type: resellerdomain.LedgerTypeOrderProfit, Amount: money.FromDecimal(decimal.NewFromInt(10)), Currency: "USD", IdempotencyKey: "order_profit:1", Status: resellerdomain.LedgerStatusPendingConfirm, AvailableAt: &past},
		{ResellerID: profile.ID, Type: resellerdomain.LedgerTypeOrderProfit, Amount: money.FromDecimal(decimal.NewFromInt(20)), Currency: "USD", IdempotencyKey: "order_profit:2", Status: resellerdomain.LedgerStatusPendingConfirm, AvailableAt: &future},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed ledger rows failed: %v", err)
	}
	affected, err := repo.MarkDueLedgerEntriesAvailable(now)
	if err != nil {
		t.Fatalf("mark due failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected affected=1, got %d", affected)
	}
	var due resellerdomain.LedgerEntry
	if err := db.First(&due, rows[0].ID).Error; err != nil {
		t.Fatalf("load due row failed: %v", err)
	}
	if due.Status != resellerdomain.LedgerStatusAvailable {
		t.Fatalf("expected due row available, got %s", due.Status)
	}
}

func TestResellerAccountingRepositoryWithdrawLocksSameCurrencyOnly(t *testing.T) {
	db := openResellerAccountingRepoTestDB(t)
	profile := seedResellerAccountingProfile(t, db)
	repo := New(db)
	now := time.Now()
	rows := []resellerdomain.LedgerEntry{
		{ResellerID: profile.ID, Type: resellerdomain.LedgerTypeOrderProfit, Amount: money.FromDecimal(decimal.NewFromInt(10)), Currency: "USD", IdempotencyKey: "order_profit:usd1", Status: resellerdomain.LedgerStatusAvailable, AvailableAt: &now},
		{ResellerID: profile.ID, Type: resellerdomain.LedgerTypeOrderProfit, Amount: money.FromDecimal(decimal.NewFromInt(20)), Currency: "CNY", IdempotencyKey: "order_profit:cny1", Status: resellerdomain.LedgerStatusAvailable, AvailableAt: &now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed ledger rows failed: %v", err)
	}
	locked, err := repo.ListAvailableLedgerEntriesForUpdate(profile.ID, "USD")
	if err != nil {
		t.Fatalf("list available ledgers failed: %v", err)
	}
	if len(locked) != 1 || locked[0].Currency != "USD" {
		t.Fatalf("expected only USD ledger, got %+v", locked)
	}
}

func TestResellerAccountingRepositoryListAdminLedgerEntriesFiltersByKeywordAndOrderNo(t *testing.T) {
	db := openResellerAccountingRepoTestDB(t)
	repo := New(db)
	profile := seedResellerAccountingProfileWithEmail(t, db, "ledger-admin@example.com")
	other := seedResellerAccountingProfileWithEmail(t, db, "other-ledger-admin@example.com")
	order := seedResellerAccountingOrder(t, db, "RADMIN-ORDER-001")
	now := time.Now().Add(-time.Hour)

	entry := resellerdomain.LedgerEntry{
		ResellerID:     profile.ID,
		OrderID:        &order.ID,
		Type:           resellerdomain.LedgerTypeOrderProfit,
		Amount:         money.FromDecimal(decimal.NewFromInt(12)),
		Currency:       "USD",
		IdempotencyKey: "admin-ledger-filter-1",
		Status:         resellerdomain.LedgerStatusAvailable,
		AvailableAt:    &now,
	}
	otherEntry := resellerdomain.LedgerEntry{
		ResellerID:     other.ID,
		Type:           resellerdomain.LedgerTypeOrderProfit,
		Amount:         money.FromDecimal(decimal.NewFromInt(8)),
		Currency:       "USD",
		IdempotencyKey: "admin-ledger-filter-2",
		Status:         resellerdomain.LedgerStatusAvailable,
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("create ledger failed: %v", err)
	}
	if err := db.Create(&otherEntry).Error; err != nil {
		t.Fatalf("create other ledger failed: %v", err)
	}

	rows, total, err := repo.ListAdminResellerLedgerEntries(resellercontract.AdminLedgerListFilter{
		Page:     1,
		PageSize: 20,
		Keyword:  "ledger-admin@example.com",
		OrderNo:  "RADMIN-ORDER-001",
		Currency: "USD",
		Status:   resellerdomain.LedgerStatusAvailable,
	})
	if err != nil {
		t.Fatalf("ListAdminResellerLedgerEntries failed: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected one ledger row, total=%d len=%d rows=%+v", total, len(rows), rows)
	}
	if rows[0].Profile == nil || rows[0].Profile.User == nil || rows[0].Profile.User.Email != "ledger-admin@example.com" {
		t.Fatalf("expected profile user preload, got %+v", rows[0].Profile)
	}
	if rows[0].Order == nil || rows[0].Order.OrderNo != "RADMIN-ORDER-001" {
		t.Fatalf("expected order preload, got %+v", rows[0].Order)
	}
}

func TestResellerAccountingRepositoryDoesNotPreloadSoftDeletedOrder(t *testing.T) {
	db := openResellerAccountingRepoTestDB(t)
	repo := New(db)
	profile := seedResellerAccountingProfileWithEmail(t, db, "deleted-order-ledger@example.com")
	order := seedResellerAccountingOrder(t, db, "DELETED-LEDGER-ORDER")
	entry := resellerdomain.LedgerEntry{
		ResellerID:     profile.ID,
		OrderID:        &order.ID,
		Type:           resellerdomain.LedgerTypeOrderProfit,
		Amount:         money.FromDecimal(decimal.NewFromInt(12)),
		Currency:       "USD",
		IdempotencyKey: "deleted-order-ledger",
		Status:         resellerdomain.LedgerStatusAvailable,
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("create ledger failed: %v", err)
	}
	deletedAt := time.Now()
	if err := db.Model(&order).Update("deleted_at", &deletedAt).Error; err != nil {
		t.Fatalf("soft delete order failed: %v", err)
	}

	rows, total, err := repo.ListAdminResellerLedgerEntries(resellercontract.AdminLedgerListFilter{
		Page:     1,
		PageSize: 20,
		OrderID:  order.ID,
	})
	if err != nil {
		t.Fatalf("ListAdminResellerLedgerEntries failed: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected ledger to remain visible, total=%d rows=%d", total, len(rows))
	}
	if rows[0].Order != nil {
		t.Fatalf("soft-deleted order leaked through preload: %+v", rows[0].Order)
	}
}

func TestResellerAccountingRepositoryListAdminBalanceAccountsFiltersAndPreloadsProfile(t *testing.T) {
	db := openResellerAccountingRepoTestDB(t)
	repo := New(db)
	profile := seedResellerAccountingProfileWithEmail(t, db, "balance-admin@example.com")
	other := seedResellerAccountingProfileWithEmail(t, db, "other-balance-admin@example.com")

	rows := []resellerdomain.BalanceAccount{
		{
			ResellerID:           profile.ID,
			Currency:             "USD",
			Status:               resellerdomain.BalanceStatusNormal,
			AvailableAmountCache: money.FromDecimal(decimal.NewFromInt(100)),
			LockedAmountCache:    money.FromDecimal(decimal.NewFromInt(10)),
			NegativeAmountCache:  money.FromDecimal(decimal.Zero),
			LastLedgerEntryID:    99,
		},
		{
			ResellerID:           other.ID,
			Currency:             "CNY",
			Status:               resellerdomain.BalanceStatusNormal,
			AvailableAmountCache: money.FromDecimal(decimal.NewFromInt(200)),
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create balance accounts failed: %v", err)
	}

	got, total, err := repo.ListAdminResellerBalanceAccounts(resellercontract.AdminBalanceAccountListFilter{
		Page:     1,
		PageSize: 20,
		Keyword:  "balance-admin@example.com",
		Currency: "USD",
		Status:   resellerdomain.BalanceStatusNormal,
	})
	if err != nil {
		t.Fatalf("ListAdminResellerBalanceAccounts failed: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("expected one balance row, total=%d len=%d rows=%+v", total, len(got), got)
	}
	if got[0].Profile == nil || got[0].Profile.User == nil || got[0].Profile.User.Email != "balance-admin@example.com" {
		t.Fatalf("expected profile user preload, got %+v", got[0].Profile)
	}
}

func TestResellerAccountingRepositoryListBalanceAccountsScopesByReseller(t *testing.T) {
	db := openResellerAccountingRepoTestDB(t)
	repo := New(db)
	profile := seedResellerAccountingProfileWithEmail(t, db, "user-balance@example.com")
	other := seedResellerAccountingProfileWithEmail(t, db, "other-user-balance@example.com")

	rows := []resellerdomain.BalanceAccount{
		{
			ResellerID:           profile.ID,
			Currency:             "USD",
			Status:               resellerdomain.BalanceStatusNormal,
			AvailableAmountCache: money.FromDecimal(decimal.RequireFromString("12.30")),
			LockedAmountCache:    money.FromDecimal(decimal.RequireFromString("1.00")),
			NegativeAmountCache:  money.FromDecimal(decimal.Zero),
			LastLedgerEntryID:    11,
		},
		{
			ResellerID:           other.ID,
			Currency:             "USD",
			Status:               resellerdomain.BalanceStatusNormal,
			AvailableAmountCache: money.FromDecimal(decimal.RequireFromString("99.00")),
		},
		{
			ResellerID:           profile.ID,
			Currency:             "EUR",
			Status:               resellerdomain.BalanceStatusDisabled,
			AvailableAmountCache: money.FromDecimal(decimal.RequireFromString("8.00")),
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create balance rows failed: %v", err)
	}

	got, total, err := repo.ListBalanceAccounts(resellercontract.BalanceAccountListFilter{
		Page:       1,
		PageSize:   20,
		ResellerID: profile.ID,
		Currency:   "USD",
		Status:     resellerdomain.BalanceStatusNormal,
	})
	if err != nil {
		t.Fatalf("ListBalanceAccounts failed: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("expected one scoped balance row, total=%d len=%d", total, len(got))
	}
	if got[0].ResellerID != profile.ID || got[0].Currency != "USD" || got[0].AvailableAmountCache.String() != "12.30" {
		t.Fatalf("unexpected balance row: %+v", got[0])
	}
}

func TestResellerAccountingRepositoryListAdminWithdrawRequestsFiltersAndPreloadsProcessor(t *testing.T) {
	db := openResellerAccountingRepoTestDB(t)
	repo := New(db)
	profile := seedResellerAccountingProfileWithEmail(t, db, "withdraw-admin@example.com")
	admin := admindomain.Admin{Username: "reviewer", PasswordHash: "hash"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin failed: %v", err)
	}
	now := time.Now()
	req := resellerdomain.WithdrawRequest{
		ResellerID:  profile.ID,
		Amount:      money.FromDecimal(decimal.NewFromInt(50)),
		Currency:    "USD",
		Channel:     "USDT",
		Account:     "TwithdrawAdmin",
		Status:      resellerdomain.WithdrawStatusPaid,
		ProcessedBy: &admin.ID,
		ProcessedAt: &now,
	}
	if err := db.Create(&req).Error; err != nil {
		t.Fatalf("create withdraw request failed: %v", err)
	}

	got, total, err := repo.ListAdminResellerWithdrawRequests(resellercontract.AdminWithdrawListFilter{
		Page:     1,
		PageSize: 20,
		Keyword:  "TwithdrawAdmin",
		Currency: "USD",
		Status:   resellerdomain.WithdrawStatusPaid,
	})
	if err != nil {
		t.Fatalf("ListAdminResellerWithdrawRequests failed: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("expected one withdraw row, total=%d len=%d rows=%+v", total, len(got), got)
	}
	if got[0].Profile == nil || got[0].Profile.User == nil || got[0].Profile.User.Email != "withdraw-admin@example.com" {
		t.Fatalf("expected profile user preload, got %+v", got[0].Profile)
	}
	if got[0].Processor == nil || got[0].Processor.Username != "reviewer" {
		t.Fatalf("expected processor preload, got %+v", got[0].Processor)
	}
}

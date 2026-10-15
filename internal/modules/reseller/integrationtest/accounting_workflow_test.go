package integrationtest

import (
	"errors"
	"fmt"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"
	"testing"
	"time"

	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	resellerapplication "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellergormstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type resellerAccountingTestHarness struct {
	query    *resellerapplication.AccountingQueryService
	withdraw *resellerapplication.AccountingWithdrawService
	ledger   *resellerapplication.AccountingLedgerService
	store    *resellergormstore.Store
}

func newResellerAccountingTestHarness(store *resellergormstore.Store, confirmDays int) resellerAccountingTestHarness {
	ledger := resellerapplication.NewAccountingLedgerService(store, confirmDays)
	return resellerAccountingTestHarness{
		query:    resellerapplication.NewAccountingQueryService(store),
		withdraw: resellerapplication.NewAccountingWithdrawService(store),
		ledger:   ledger,
		store:    store,
	}
}

func openResellerAccountingServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:reseller_accounting_service_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&paymentdomain.Payment{},
		&paymentdomain.PaymentChannel{},
		&orderdomain.OrderRefundRecord{},
		&resellerdomain.Profile{},
		&resellerdomain.OrderSnapshot{},
		&resellerdomain.LedgerEntry{},
		&resellerdomain.WithdrawRequest{},
		&resellerdomain.BalanceAccount{},
		&affiliatedomain.Profile{}, &walletdomain.Account{}, &walletdomain.Transaction{}, &walletdomain.EarningSettlement{}, &walletdomain.WithdrawRequest{},
	); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return db
}

func seedResellerAccountingProfile(t *testing.T, db *gorm.DB) resellerdomain.Profile {
	t.Helper()
	user := userdomain.User{Email: fmt.Sprintf("reseller-%s@example.test", uuid.NewString()), PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create reseller user failed: %v", err)
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

func TestResellerAccountingServiceListAdminWithdrawRequests(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 7)
	profile := seedResellerAccountingProfile(t, db)
	req := resellerdomain.WithdrawRequest{
		ResellerID: profile.ID,
		Amount:     money.FromDecimal(decimal.NewFromInt(25)),
		Currency:   "USD",
		Channel:    "USDT",
		Account:    "TserviceWithdraw",
		Status:     resellerdomain.WithdrawStatusPending,
	}
	if err := db.Create(&req).Error; err != nil {
		t.Fatalf("create withdraw failed: %v", err)
	}

	rows, total, err := svc.query.ListAdminWithdrawRequests(resellercontract.AdminWithdrawListFilter{
		Page:       1,
		PageSize:   20,
		ResellerID: profile.ID,
		Currency:   " USD ",
		Status:     " pending ",
	})
	if err != nil {
		t.Fatalf("ListAdminWithdrawRequests failed: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ID != req.ID {
		t.Fatalf("expected created withdraw, total=%d rows=%+v", total, rows)
	}
}

func TestResellerAccountingServiceGetUserFinanceDashboardScopesToUserProfile(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)
	profile := seedResellerAccountingProfile(t, db)
	other := seedResellerAccountingProfile(t, db)
	if err := db.Create(&resellerdomain.BalanceAccount{
		ResellerID:           profile.ID,
		Currency:             "USD",
		Status:               resellerdomain.BalanceStatusNormal,
		AvailableAmountCache: money.FromDecimal(decimal.RequireFromString("18.50")),
	}).Error; err != nil {
		t.Fatalf("create balance failed: %v", err)
	}
	if err := db.Create(&resellerdomain.BalanceAccount{
		ResellerID:           other.ID,
		Currency:             "USD",
		Status:               resellerdomain.BalanceStatusNormal,
		AvailableAmountCache: money.FromDecimal(decimal.RequireFromString("99.00")),
	}).Error; err != nil {
		t.Fatalf("create other balance failed: %v", err)
	}

	dashboard, err := svc.query.GetUserFinanceDashboard(profile.UserID)
	if err != nil {
		t.Fatalf("GetUserFinanceDashboard failed: %v", err)
	}
	if !dashboard.Opened || dashboard.Profile == nil || dashboard.Profile.ID != profile.ID {
		t.Fatalf("expected opened dashboard for profile %d, got %+v", profile.ID, dashboard)
	}
	if dashboard.WithdrawEnabled || dashboard.WithdrawDisabledReason != "unified_wallet" {
		t.Fatalf("expected active normal profile withdraw enabled, got %+v", dashboard)
	}
	if len(dashboard.Balances) != 1 || dashboard.Balances[0].AvailableAmountCache.String() != "18.50" {
		t.Fatalf("expected scoped balances, got %+v", dashboard.Balances)
	}
}

func TestResellerAccountingServiceGetUserFinanceDashboardMarksWithdrawUnavailable(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)

	inactive := seedResellerAccountingProfile(t, db)
	inactive.Status = resellerdomain.ProfileStatusDisabled
	if err := db.Save(&inactive).Error; err != nil {
		t.Fatalf("disable profile failed: %v", err)
	}
	inactiveDashboard, err := svc.query.GetUserFinanceDashboard(inactive.UserID)
	if err != nil {
		t.Fatalf("GetUserFinanceDashboard inactive failed: %v", err)
	}
	if !inactiveDashboard.Opened || inactiveDashboard.WithdrawEnabled || inactiveDashboard.WithdrawDisabledReason != resellercontract.WithdrawDisabledReasonProfileInactive {
		t.Fatalf("expected inactive profile withdraw disabled, got %+v", inactiveDashboard)
	}

	frozen := seedResellerAccountingProfile(t, db)
	frozen.SettlementStatus = resellerdomain.SettlementStatusFrozen
	if err := db.Save(&frozen).Error; err != nil {
		t.Fatalf("freeze settlement failed: %v", err)
	}
	frozenDashboard, err := svc.query.GetUserFinanceDashboard(frozen.UserID)
	if err != nil {
		t.Fatalf("GetUserFinanceDashboard frozen failed: %v", err)
	}
	if !frozenDashboard.Opened || frozenDashboard.WithdrawEnabled || frozenDashboard.WithdrawDisabledReason != resellercontract.WithdrawDisabledReasonSettlementUnavailable {
		t.Fatalf("expected frozen settlement withdraw disabled, got %+v", frozenDashboard)
	}
}

func TestResellerAccountingServiceApplyUserWithdrawRequiresActiveNormalProfile(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)
	profile := seedResellerAccountingProfile(t, db)
	profile.Status = resellerdomain.ProfileStatusDisabled
	if err := db.Save(&profile).Error; err != nil {
		t.Fatalf("disable profile failed: %v", err)
	}

	_, err := svc.withdraw.ApplyUserWithdraw(profile.UserID, resellercontract.WithdrawApplyInput{
		Amount:   decimal.NewFromInt(10),
		Currency: "USD",
		Channel:  "USDT",
		Account:  "T-address",
	})
	if !errors.Is(err, resellercontract.ErrProfileInactive) {
		t.Fatalf("expected ErrProfileInactive, got %v", err)
	}
}

func seedPaidResellerOrderSnapshot(t *testing.T, db *gorm.DB, eligible bool) (orderdomain.Order, paymentdomain.Payment, resellerdomain.OrderSnapshot) {
	t.Helper()
	user := userdomain.User{Email: fmt.Sprintf("buyer-%d@example.test", time.Now().UnixNano()), PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	profile := resellerdomain.Profile{UserID: user.ID, Status: resellerdomain.ProfileStatusActive, SettlementStatus: resellerdomain.SettlementStatusNormal}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	resellerID := profile.ID
	now := time.Now()
	order := orderdomain.Order{
		OrderNo:              fmt.Sprintf("DJ-RES-%d", now.UnixNano()),
		UserID:               user.ID,
		Status:               constants.OrderStatusPaid,
		TotalAmount:          money.FromDecimal(decimal.NewFromInt(130)),
		OriginalAmount:       money.FromDecimal(decimal.NewFromInt(130)),
		Currency:             "USD",
		WalletPaidAmount:     money.FromDecimal(decimal.NewFromInt(30)),
		OnlinePaidAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		ResellerID:           &resellerID,
		ResellerDomain:       "shop.example.test",
		ResellerProfitAmount: money.FromDecimal(decimal.NewFromInt(30)),
		PaidAt:               &now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	channel := paymentdomain.PaymentChannel{
		Name:         "Stripe",
		ProviderType: constants.PaymentProviderOfficial,
		ChannelType:  constants.PaymentChannelTypeStripe,
		IsActive:     true,
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create payment channel failed: %v", err)
	}
	payment := paymentdomain.Payment{
		OrderID:   order.ID,
		ChannelID: channel.ID,
		Status:    constants.PaymentStatusSuccess,
		Amount:    money.FromDecimal(decimal.NewFromInt(100)),
		Currency:  "USD",
		PaidAt:    &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}
	snapshot := resellerdomain.OrderSnapshot{
		OrderID:           order.ID,
		ResellerID:        profile.ID,
		Domain:            "shop.example.test",
		Currency:          "USD",
		ResellerUserID:    profile.UserID,
		BuyerUserID:       user.ID,
		BaseAmount:        money.FromDecimal(decimal.NewFromInt(100)),
		ResellerAmount:    money.FromDecimal(decimal.NewFromInt(130)),
		ProfitAmount:      money.FromDecimal(decimal.NewFromInt(30)),
		ProfitEligible:    eligible,
		ProfitBlockReason: "",
		PricingSnapshotJSON: jsonmap.JSON{
			"base_amount":     "100.00",
			"reseller_amount": "130.00",
			"profit_amount":   "30.00",
			"items": []interface{}{
				map[string]interface{}{
					"order_item_id":         "1",
					"product_id":            "10",
					"sku_id":                "100",
					"quantity":              "2",
					"base_total_amount":     "100.00",
					"reseller_total_amount": "130.00",
					"profit_amount":         "30.00",
				},
			},
		},
		RiskSnapshotJSON: jsonmap.JSON{"profit_eligible": eligible},
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if !eligible {
		snapshot.ProfitBlockReason = "self_dealing_owner"
	}
	if err := db.Create(&snapshot).Error; err != nil {
		t.Fatalf("create snapshot failed: %v", err)
	}
	if !eligible {
		if err := db.Model(&snapshot).Update("profit_eligible", false).Error; err != nil {
			t.Fatalf("force snapshot profit_eligible=false failed: %v", err)
		}
		snapshot.ProfitEligible = false
	}
	return order, payment, snapshot
}

func TestResellerAccountingPostOrderProfitIdempotent(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	order, payment, snapshot := seedPaidResellerOrderSnapshot(t, db, true)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 7)
	err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.PostOrderProfit(svc.store.BindTx(tx), &order, &payment)
	})
	if err != nil {
		t.Fatalf("first post failed: %v", err)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.PostOrderProfit(svc.store.BindTx(tx), &order, &payment)
	})
	if err != nil {
		t.Fatalf("second post failed: %v", err)
	}
	var rows []resellerdomain.LedgerEntry
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("list ledger failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one ledger row, got %d", len(rows))
	}
	if rows[0].ResellerID != snapshot.ResellerID || rows[0].Amount.String() != "30.00" || rows[0].Currency != "USD" {
		t.Fatalf("unexpected ledger row: %+v", rows[0])
	}
	if rows[0].Status != resellerdomain.LedgerStatusPendingConfirm {
		t.Fatalf("expected pending_confirm, got %s", rows[0].Status)
	}
	if rows[0].AvailableAt == nil || rows[0].AvailableAt.Before(time.Now().Add(6*24*time.Hour)) {
		t.Fatalf("expected available_at roughly 7 days later, got %v", rows[0].AvailableAt)
	}
}

func TestResellerAccountingSkipsSelfDealingSnapshot(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	order, payment, _ := seedPaidResellerOrderSnapshot(t, db, false)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 7)
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.PostOrderProfit(svc.store.BindTx(tx), &order, &payment)
	}); err != nil {
		t.Fatalf("post self-dealing order failed: %v", err)
	}
	var count int64
	if err := db.Model(&resellerdomain.LedgerEntry{}).Count(&count).Error; err != nil {
		t.Fatalf("count ledger failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no ledger row for self-dealing, got %d", count)
	}
}

func TestResellerAccountingMissingSnapshotSkipsWithoutRollingBack(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	order, payment, snapshot := seedPaidResellerOrderSnapshot(t, db, true)
	if err := db.Delete(&snapshot).Error; err != nil {
		t.Fatalf("delete snapshot failed: %v", err)
	}
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 7)
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.PostOrderProfit(svc.store.BindTx(tx), &order, &payment)
	}); err != nil {
		t.Fatalf("post order profit with missing snapshot should skip, got %v", err)
	}
	var count int64
	if err := db.Model(&resellerdomain.LedgerEntry{}).Count(&count).Error; err != nil {
		t.Fatalf("count ledger failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no ledger row for missing snapshot, got %d", count)
	}
}

func TestResellerAccountingConfirmDueLedgerEntries(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	order, payment, _ := seedPaidResellerOrderSnapshot(t, db, true)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.PostOrderProfit(svc.store.BindTx(tx), &order, &payment)
	}); err != nil {
		t.Fatalf("post order profit failed: %v", err)
	}
	affected, err := svc.ledger.ConfirmDueLedgerEntries(time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("confirm due failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected affected=1, got %d", affected)
	}
	var row resellerdomain.LedgerEntry
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("load ledger failed: %v", err)
	}
	if row.Status != resellerdomain.LedgerStatusAvailable {
		t.Fatalf("expected available, got %s", row.Status)
	}
}

func TestPaymentSuccessTransactionPostsResellerLedger(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	order, payment, _ := seedPaidResellerOrderSnapshot(t, db, true)
	order.Status = constants.OrderStatusPendingPayment
	order.PaidAt = nil
	if err := db.Save(&order).Error; err != nil {
		t.Fatalf("reset order failed: %v", err)
	}
	payment.Status = constants.PaymentStatusPending
	payment.PaidAt = nil
	if err := db.Save(&payment).Error; err != nil {
		t.Fatalf("reset payment failed: %v", err)
	}
	repo := resellergormstore.New(db)
	accounting := newResellerAccountingTestHarness(repo, 0)
	orderRepo := ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	paymentRepo := paymentgormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	productRepo := productgormstore.NewProductStore(db)
	productSKURepo := productgormstore.NewSKUStore(db)
	paymentSvc := paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{
		OrderStore:         orderRepo,
		PaymentStore:       paymentRepo,
		ProductRepo:        productRepo,
		ProductSKURepo:     productSKURepo,
		ResellerAccounting: accounting.ledger,
	})
	updated, err := paymentSvc.HandleCallback(paymentapp.PaymentCallbackInput{
		PaymentID:   payment.ID,
		OrderNo:     order.OrderNo,
		ChannelID:   payment.ChannelID,
		Status:      constants.PaymentStatusSuccess,
		ProviderRef: payment.ProviderRef,
		Amount:      payment.Amount,
		Currency:    payment.Currency,
	})
	if err != nil {
		t.Fatalf("handle payment callback failed: %v", err)
	}
	if updated == nil || updated.Status != constants.PaymentStatusSuccess {
		t.Fatalf("expected successful payment, got %+v", updated)
	}
	if err := db.First(&order, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if order.Status != constants.OrderStatusPaid {
		t.Fatalf("expected paid order, got %s", order.Status)
	}
	var count int64
	if err := db.Model(&resellerdomain.LedgerEntry{}).Where("idempotency_key = ?", fmt.Sprintf("order_profit:%d", order.ID)).Count(&count).Error; err != nil {
		t.Fatalf("count reseller ledger failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected reseller ledger created, got %d", count)
	}
}

func TestResellerAccountingRefundDeductUsesSnapshotItems(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	order, payment, snapshot := seedPaidResellerOrderSnapshot(t, db, true)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.PostOrderProfit(svc.store.BindTx(tx), &order, &payment)
	}); err != nil {
		t.Fatalf("post profit failed: %v", err)
	}
	refundRecord := orderdomain.OrderRefundRecord{
		UserID:    order.UserID,
		OrderID:   order.ID,
		Type:      constants.OrderRefundTypeManual,
		Amount:    money.FromDecimal(decimal.NewFromInt(65)),
		Currency:  "USD",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.Create(&refundRecord).Error; err != nil {
		t.Fatalf("create refund record failed: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.HandleRefundDeduct(svc.store.BindTx(tx), &order, &refundRecord, decimal.Zero)
	}); err != nil {
		t.Fatalf("refund deduct failed: %v", err)
	}
	var deduct resellerdomain.LedgerEntry
	if err := db.Where("idempotency_key = ?", fmt.Sprintf("refund_deduct:%d", refundRecord.ID)).First(&deduct).Error; err != nil {
		t.Fatalf("load deduct ledger failed: %v", err)
	}
	if deduct.ResellerID != snapshot.ResellerID || deduct.Type != resellerdomain.LedgerTypeRefundDeduct || deduct.Currency != "USD" {
		t.Fatalf("unexpected deduct row: %+v", deduct)
	}
	if deduct.Amount.String() != "-15.00" {
		t.Fatalf("expected half profit deduction -15.00, got %s", deduct.Amount.String())
	}
	if _, ok := deduct.MetadataJSON["refund_allocation_json"]; !ok {
		t.Fatalf("expected refund_allocation_json metadata, got %+v", deduct.MetadataJSON)
	}
}

func TestResellerAccountingRefundDeductIsIdempotent(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	order, payment, _ := seedPaidResellerOrderSnapshot(t, db, true)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.PostOrderProfit(svc.store.BindTx(tx), &order, &payment)
	}); err != nil {
		t.Fatalf("post profit failed: %v", err)
	}
	refundRecord := orderdomain.OrderRefundRecord{UserID: order.UserID, OrderID: order.ID, Type: constants.OrderRefundTypeManual, Amount: money.FromDecimal(decimal.NewFromInt(65)), Currency: "USD"}
	if err := db.Create(&refundRecord).Error; err != nil {
		t.Fatalf("create refund record failed: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := db.Transaction(func(tx *gorm.DB) error {
			return svc.ledger.HandleRefundDeduct(svc.store.BindTx(tx), &order, &refundRecord, decimal.Zero)
		}); err != nil {
			t.Fatalf("refund deduct attempt %d failed: %v", i+1, err)
		}
	}
	var count int64
	if err := db.Model(&resellerdomain.LedgerEntry{}).Where("type = ?", resellerdomain.LedgerTypeRefundDeduct).Count(&count).Error; err != nil {
		t.Fatalf("count deduct failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one refund deduct row, got %d", count)
	}
}

func TestResellerAccountingRefundDeductSkipsIneligibleSnapshot(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	order, _, _ := seedPaidResellerOrderSnapshot(t, db, false)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)
	refundRecord := orderdomain.OrderRefundRecord{UserID: order.UserID, OrderID: order.ID, Type: constants.OrderRefundTypeManual, Amount: money.FromDecimal(decimal.NewFromInt(65)), Currency: "USD"}
	if err := db.Create(&refundRecord).Error; err != nil {
		t.Fatalf("create refund record failed: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.HandleRefundDeduct(svc.store.BindTx(tx), &order, &refundRecord, decimal.Zero)
	}); err != nil {
		t.Fatalf("refund deduct for ineligible snapshot failed: %v", err)
	}
	var count int64
	if err := db.Model(&resellerdomain.LedgerEntry{}).Where("type = ?", resellerdomain.LedgerTypeRefundDeduct).Count(&count).Error; err != nil {
		t.Fatalf("count refund deduct failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no refund deduct row for ineligible snapshot, got %d", count)
	}
}

func TestResellerAccountingRefundDeductMissingSnapshotSkipsWithoutRollingBack(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	order, _, snapshot := seedPaidResellerOrderSnapshot(t, db, true)
	if err := db.Delete(&snapshot).Error; err != nil {
		t.Fatalf("delete snapshot failed: %v", err)
	}
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)
	refundRecord := orderdomain.OrderRefundRecord{UserID: order.UserID, OrderID: order.ID, Type: constants.OrderRefundTypeManual, Amount: money.FromDecimal(decimal.NewFromInt(65)), Currency: "USD"}
	if err := db.Create(&refundRecord).Error; err != nil {
		t.Fatalf("create refund record failed: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.HandleRefundDeduct(svc.store.BindTx(tx), &order, &refundRecord, decimal.Zero)
	}); err != nil {
		t.Fatalf("refund deduct with missing snapshot should skip, got %v", err)
	}
	var count int64
	if err := db.Model(&resellerdomain.LedgerEntry{}).Where("type = ?", resellerdomain.LedgerTypeRefundDeduct).Count(&count).Error; err != nil {
		t.Fatalf("count refund deduct failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no refund deduct row for missing snapshot, got %d", count)
	}
}

func TestLegacyResellerWithdrawCannotDoubleSpendWalletEarnings(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	profile := seedResellerAccountingProfile(t, db)
	row := resellerdomain.LedgerEntry{ResellerID: profile.ID, Type: resellerdomain.LedgerTypeOrderProfit, Amount: money.FromDecimal(decimal.NewFromInt(60)), Currency: "CNY", IdempotencyKey: "legacy-blocked", Status: resellerdomain.LedgerStatusAvailable}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	svc := newResellerAccountingTestHarness(resellergormstore.New(db), 0)
	if _, err := svc.withdraw.ApplyWithdraw(profile.ID, resellercontract.WithdrawApplyInput{Amount: decimal.NewFromInt(20), Currency: "CNY", Channel: "wechat", Account: "test"}); !errors.Is(err, walletcontract.ErrUnifiedWallet) {
		t.Fatal(err)
	}
	var requests int64
	db.Model(&resellerdomain.WithdrawRequest{}).Count(&requests)
	if requests != 0 {
		t.Fatal("legacy endpoint created a withdrawal")
	}
	db.First(&row, row.ID)
	if row.Status != resellerdomain.LedgerStatusAvailable || row.WithdrawRequestID != nil {
		t.Fatal("legacy endpoint changed an earnings ledger")
	}
}

func seedLegacyWithdraw(t *testing.T, db *gorm.DB, row *resellerdomain.LedgerEntry, amount int64) (*resellerdomain.WithdrawRequest, error) {
	t.Helper()
	request := &resellerdomain.WithdrawRequest{ResellerID: row.ResellerID, Amount: money.FromDecimal(decimal.NewFromInt(amount)), Currency: row.Currency, Channel: "usdt", Account: "historical-test", Status: resellerdomain.WithdrawStatusPending}
	if err := db.Create(request).Error; err != nil {
		return nil, err
	}
	remaining := row.Amount.Decimal.Sub(request.Amount.Decimal)
	if remaining.IsPositive() {
		extra := *row
		extra.ID = 0
		extra.Amount = money.FromDecimal(remaining)
		extra.IdempotencyKey += "-remainder"
		if err := db.Create(&extra).Error; err != nil {
			return nil, err
		}
	}
	row.Amount = request.Amount
	row.Status = resellerdomain.LedgerStatusLocked
	row.WithdrawRequestID = &request.ID
	if err := db.Save(row).Error; err != nil {
		return nil, err
	}
	return request, nil
}

func TestResellerAccountingRejectWithdrawUnlocksLedgers(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	profile := seedResellerAccountingProfile(t, db)
	now := time.Now()
	row := resellerdomain.LedgerEntry{ResellerID: profile.ID, Type: resellerdomain.LedgerTypeOrderProfit, Amount: money.FromDecimal(decimal.NewFromInt(10)), Currency: "USD", IdempotencyKey: "order_profit:reject", Status: resellerdomain.LedgerStatusAvailable, AvailableAt: &now}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed ledger failed: %v", err)
	}
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)
	req, err := seedLegacyWithdraw(t, db, &row, 10)
	if err != nil {
		t.Fatalf("apply withdraw failed: %v", err)
	}
	reviewed, err := svc.withdraw.ReviewWithdraw(99, req.ID, resellercontract.WithdrawActionReject, "bad account")
	if err != nil {
		t.Fatalf("reject withdraw failed: %v", err)
	}
	if reviewed.Status != resellerdomain.WithdrawStatusRejected {
		t.Fatalf("expected rejected, got %s", reviewed.Status)
	}
	var unlocked resellerdomain.LedgerEntry
	if err := db.First(&unlocked, row.ID).Error; err != nil {
		t.Fatalf("load ledger failed: %v", err)
	}
	if unlocked.Status != resellerdomain.LedgerStatusAvailable || unlocked.WithdrawRequestID != nil {
		t.Fatalf("expected unlocked available ledger, got %+v", unlocked)
	}
}

func TestResellerAccountingPayWithdrawMarksLedgersWithdrawn(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	profile := seedResellerAccountingProfile(t, db)
	now := time.Now()
	row := resellerdomain.LedgerEntry{ResellerID: profile.ID, Type: resellerdomain.LedgerTypeOrderProfit, Amount: money.FromDecimal(decimal.NewFromInt(10)), Currency: "USD", IdempotencyKey: "order_profit:pay", Status: resellerdomain.LedgerStatusAvailable, AvailableAt: &now}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed ledger failed: %v", err)
	}
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)
	req, err := seedLegacyWithdraw(t, db, &row, 10)
	if err != nil {
		t.Fatalf("apply withdraw failed: %v", err)
	}
	reviewed, err := svc.withdraw.ReviewWithdraw(99, req.ID, resellercontract.WithdrawActionPay, "")
	if err != nil {
		t.Fatalf("pay withdraw failed: %v", err)
	}
	if reviewed.Status != resellerdomain.WithdrawStatusPaid {
		t.Fatalf("expected paid, got %s", reviewed.Status)
	}
	var withdrawn resellerdomain.LedgerEntry
	if err := db.First(&withdrawn, row.ID).Error; err != nil {
		t.Fatalf("load ledger failed: %v", err)
	}
	if withdrawn.Status != resellerdomain.LedgerStatusWithdrawn || withdrawn.WithdrawRequestID == nil || *withdrawn.WithdrawRequestID != req.ID {
		t.Fatalf("expected withdrawn ledger, got %+v", withdrawn)
	}
	var balance resellerdomain.BalanceAccount
	if err := db.Where("reseller_id = ? AND currency = ?", profile.ID, "USD").First(&balance).Error; err != nil {
		t.Fatalf("load balance failed: %v", err)
	}
	if balance.AvailableAmountCache.String() != "0.00" || balance.LockedAmountCache.String() != "0.00" || balance.NegativeAmountCache.String() != "0.00" || balance.Status != resellerdomain.BalanceStatusNormal {
		t.Fatalf("expected zero normal balance after full paid withdraw, got %+v", balance)
	}
}

func TestResellerAccountingPayPartialWithdrawKeepsRemainingAvailableBalance(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	profile := seedResellerAccountingProfile(t, db)
	now := time.Now()
	row := resellerdomain.LedgerEntry{ResellerID: profile.ID, Type: resellerdomain.LedgerTypeOrderProfit, Amount: money.FromDecimal(decimal.NewFromInt(60)), Currency: "USD", IdempotencyKey: "order_profit:pay-partial", Status: resellerdomain.LedgerStatusAvailable, AvailableAt: &now}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed ledger failed: %v", err)
	}
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)
	req, err := seedLegacyWithdraw(t, db, &row, 25)
	if err != nil {
		t.Fatalf("apply withdraw failed: %v", err)
	}
	if _, err := svc.withdraw.ReviewWithdraw(99, req.ID, resellercontract.WithdrawActionPay, ""); err != nil {
		t.Fatalf("pay withdraw failed: %v", err)
	}
	var balance resellerdomain.BalanceAccount
	if err := db.Where("reseller_id = ? AND currency = ?", profile.ID, "USD").First(&balance).Error; err != nil {
		t.Fatalf("load balance failed: %v", err)
	}
	if balance.AvailableAmountCache.String() != "35.00" || balance.LockedAmountCache.String() != "0.00" || balance.NegativeAmountCache.String() != "0.00" || balance.Status != resellerdomain.BalanceStatusNormal {
		t.Fatalf("expected remaining available balance 35.00 after partial paid withdraw, got %+v", balance)
	}
}

func TestResellerAccountingRefundDeductDefersWhileProfitPending(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	order, payment, snapshot := seedPaidResellerOrderSnapshot(t, db, true)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 7)

	// 利润先入账，处于 pending_confirm（尚未到账）。
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.PostOrderProfit(svc.store.BindTx(tx), &order, &payment)
	}); err != nil {
		t.Fatalf("post profit failed: %v", err)
	}

	// 确认窗口内发生退款（退一半 65/130），扣减利润 15。
	refundRecord := orderdomain.OrderRefundRecord{UserID: order.UserID, OrderID: order.ID, Type: constants.OrderRefundTypeManual, Amount: money.FromDecimal(decimal.NewFromInt(65)), Currency: "USD"}
	if err := db.Create(&refundRecord).Error; err != nil {
		t.Fatalf("create refund record failed: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.HandleRefundDeduct(svc.store.BindTx(tx), &order, &refundRecord, decimal.Zero)
	}); err != nil {
		t.Fatalf("refund deduct failed: %v", err)
	}

	// 扣减流水应与待确认利润对齐：pending_confirm 且带到账时间。
	var deduct resellerdomain.LedgerEntry
	if err := db.Where("idempotency_key = ?", fmt.Sprintf("refund_deduct:%d", refundRecord.ID)).First(&deduct).Error; err != nil {
		t.Fatalf("load deduct ledger failed: %v", err)
	}
	if deduct.Status != resellerdomain.LedgerStatusPendingConfirm {
		t.Fatalf("expected deduct pending_confirm while profit pending, got %s", deduct.Status)
	}
	if deduct.AvailableAt == nil {
		t.Fatalf("expected deduct available_at set when pending, got nil")
	}
	if deduct.Amount.String() != "-15.00" {
		t.Fatalf("expected deduct -15.00, got %s", deduct.Amount.String())
	}

	// 关键回归：未到账利润的退款不得把账户算成负余额 / 冻结。
	var balance resellerdomain.BalanceAccount
	if err := db.Where("reseller_id = ? AND currency = ?", snapshot.ResellerID, "USD").First(&balance).Error; err != nil {
		t.Fatalf("load balance failed: %v", err)
	}
	if balance.AvailableAmountCache.String() != "0.00" || balance.NegativeAmountCache.String() != "0.00" || balance.Status != resellerdomain.BalanceStatusNormal {
		t.Fatalf("expected normal zero balance while profit pending, got %+v", balance)
	}

	// 到期确认后，利润与扣减同步转为可用，净额 30 - 15 = 15。
	if _, err := svc.ledger.ConfirmDueLedgerEntries(time.Now().Add(8 * 24 * time.Hour)); err != nil {
		t.Fatalf("confirm due failed: %v", err)
	}
	available, err := repo.SumLedgerAmount(snapshot.ResellerID, "USD", []string{resellerdomain.LedgerStatusAvailable})
	if err != nil {
		t.Fatalf("sum available failed: %v", err)
	}
	if available.StringFixed(2) != "15.00" {
		t.Fatalf("expected net available 15.00 after confirm, got %s", available.StringFixed(2))
	}

	// 确认后余额缓存应同步刷新（此前 confirm 仅改状态、不刷新缓存，会长期停留在 0）。
	var confirmed resellerdomain.BalanceAccount
	if err := db.Where("reseller_id = ? AND currency = ?", snapshot.ResellerID, "USD").First(&confirmed).Error; err != nil {
		t.Fatalf("load confirmed balance failed: %v", err)
	}
	if confirmed.AvailableAmountCache.String() != "15.00" || confirmed.NegativeAmountCache.String() != "0.00" || confirmed.Status != resellerdomain.BalanceStatusNormal {
		t.Fatalf("expected refreshed available cache 15.00 after confirm, got %+v", confirmed)
	}
}

// TestResellerAccountingRefundDeductDoesNotOverDeductAcrossPartialRefunds 验证多次部分退款的
// 累计利润扣减恰好等于原始利润、不会超扣（修复前按递减剩余额累计会扣成 42）。
func TestResellerAccountingRefundDeductDoesNotOverDeductAcrossPartialRefunds(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	order, payment, _ := seedPaidResellerOrderSnapshot(t, db, true)
	repo := resellergormstore.New(db)
	svc := newResellerAccountingTestHarness(repo, 0)
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.PostOrderProfit(svc.store.BindTx(tx), &order, &payment)
	}); err != nil {
		t.Fatalf("post profit failed: %v", err)
	}

	// 第一次部分退款 52/130，扣减利润 30 * 0.4 = 12。
	refund1 := orderdomain.OrderRefundRecord{UserID: order.UserID, OrderID: order.ID, Type: constants.OrderRefundTypeManual, Amount: money.FromDecimal(decimal.NewFromInt(52)), Currency: "USD"}
	if err := db.Create(&refund1).Error; err != nil {
		t.Fatalf("create refund1 failed: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.HandleRefundDeduct(svc.store.BindTx(tx), &order, &refund1, decimal.Zero)
	}); err != nil {
		t.Fatalf("refund deduct 1 failed: %v", err)
	}

	// 第二次退款 78（剩余全部），refundedBefore=52，订单转为全额退款。
	refund2 := orderdomain.OrderRefundRecord{UserID: order.UserID, OrderID: order.ID, Type: constants.OrderRefundTypeManual, Amount: money.FromDecimal(decimal.NewFromInt(78)), Currency: "USD"}
	if err := db.Create(&refund2).Error; err != nil {
		t.Fatalf("create refund2 failed: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.ledger.HandleRefundDeduct(svc.store.BindTx(tx), &order, &refund2, decimal.NewFromInt(52))
	}); err != nil {
		t.Fatalf("refund deduct 2 failed: %v", err)
	}

	totalDeduct, err := repo.SumLedgerAmountByOrderAndType(order.ID, resellerdomain.LedgerTypeRefundDeduct)
	if err != nil {
		t.Fatalf("sum deduct failed: %v", err)
	}
	if totalDeduct.StringFixed(2) != "-30.00" {
		t.Fatalf("expected cumulative deduct -30.00 (== original profit), got %s", totalDeduct.StringFixed(2))
	}
}

// Negative source entries must reduce the unified wallet, not just its display cache.
type walletTestPolicy struct{}

func (walletTestPolicy) GetWalletWithdrawPolicy() (walletcontract.WithdrawPolicy, error) {
	return walletcontract.WithdrawPolicy{Enabled: true, Minimum: money.FromDecimal(decimal.NewFromInt(20)), Channels: []string{"wechat"}, Currency: "CNY"}, nil
}
func TestResellerNetIncomeTransfersOnceToUnifiedWallet(t *testing.T) {
	db := openResellerAccountingServiceTestDB(t)
	raw, _ := db.DB()
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { raw.Close() })
	profile := seedResellerAccountingProfile(t, db)
	repo := resellergormstore.New(db)
	for i, amount := range []int64{100, -50} {
		kind := resellerdomain.LedgerTypeOrderProfit
		if amount < 0 {
			kind = resellerdomain.LedgerTypeRefundDeduct
		}
		if err := db.Create(&resellerdomain.LedgerEntry{ResellerID: profile.ID, Type: kind, Amount: money.FromDecimal(decimal.NewFromInt(amount)), Currency: "CNY", IdempotencyKey: fmt.Sprintf("net-%d", i), Status: resellerdomain.LedgerStatusAvailable}).Error; err != nil {
			t.Fatal(err)
		}
	}
	refresh := func() {
		t.Helper()
		if err := db.Transaction(func(tx *gorm.DB) error {
			return resellerapplication.RefreshBalanceAccount(repo.BindTx(tx), profile.ID, "CNY", time.Now())
		}); err != nil {
			t.Fatal(err)
		}
	}
	refresh()
	refresh()
	ws := walletstore.New(db)
	svc := walletapp.NewService(walletapp.Options{Repository: ws, Transactions: ws, WithdrawPolicy: walletTestPolicy{}})
	a, err := svc.GetAccount(profile.UserID)
	if err != nil || a.WithdrawableBalance.String() != "50.00" || a.Balance.String() != "50.00" {
		t.Fatalf("net earnings: %+v %v", a, err)
	}
	input := walletcontract.WithdrawInput{UserID: profile.UserID, Amount: money.FromDecimal(decimal.NewFromInt(80)), Channel: "wechat", Account: "local-simulated-recipient", RealName: "Local Test", RequestKey: "net-income-wallet-01"}
	if _, err := svc.ApplyWithdraw(input); !errors.Is(err, walletcontract.ErrWithdrawInsufficient) {
		t.Fatalf("over-net withdrawal: %v", err)
	}
	input.Amount = money.FromDecimal(decimal.NewFromInt(50))
	request, err := svc.ApplyWithdraw(input)
	if err != nil {
		t.Fatal(err)
	}
	refresh()
	a, _ = svc.GetAccount(profile.UserID)
	if a.Balance.String() != "0.00" || a.FrozenBalance.String() != "50.00" {
		t.Fatalf("settlement replay minted earnings: %+v", a)
	}
	if err := db.Create(&resellerdomain.LedgerEntry{ResellerID: profile.ID, Type: resellerdomain.LedgerTypeRefundDeduct, Amount: money.FromDecimal(decimal.NewFromInt(-50)), Currency: "CNY", IdempotencyKey: "net-final-refund", Status: resellerdomain.LedgerStatusAvailable}).Error; err != nil {
		t.Fatal(err)
	}
	refresh()
	a, _ = svc.GetAccount(profile.UserID)
	if a.EarningsDebt.String() != "50.00" {
		t.Fatalf("missing refund recovery: %+v", a)
	}
	if _, err := svc.ReviewWithdraw(walletcontract.WithdrawReviewInput{ID: request.ID, AdminID: 1, Action: "reject", Remark: "Refund reversed income"}); err != nil {
		t.Fatal(err)
	}
	a, _ = svc.GetAccount(profile.UserID)
	if a.EarningsDebt.String() != "0.00" || a.Balance.String() != "0.00" || a.FrozenBalance.String() != "0.00" {
		t.Fatalf("refund release failed: %+v", a)
	}
}

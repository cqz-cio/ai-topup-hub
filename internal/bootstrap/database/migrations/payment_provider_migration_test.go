package migrations

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupPaymentProviderRenameTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:payment_provider_rename_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	gormdb.DB = db
	if err := db.AutoMigrate(&paymentdomain.PaymentChannel{}, &paymentdomain.Payment{}, &settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}

func TestEnsurePaymentFeePolicyMigrationClassifiesHistoryAndIsIdempotent(t *testing.T) {
	db := setupPaymentProviderRenameTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	payments := []paymentdomain.Payment{
		{
			OrderID: 1, ProviderType: constants.PaymentProviderOfficial, ChannelType: constants.PaymentChannelTypeAlipay,
			InteractionMode: constants.PaymentInteractionRedirect, Amount: money.FromDecimal(decimal.NewFromInt(103)),
			FeeAmount: money.FromDecimal(decimal.NewFromInt(3)), Currency: "CNY", Status: constants.PaymentStatusSuccess,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			OrderID: 2, ProviderType: constants.PaymentProviderOfficial, ChannelType: constants.PaymentChannelTypeWechat,
			InteractionMode: constants.PaymentInteractionQR, Amount: money.FromDecimal(decimal.NewFromInt(100)),
			FeeAmount: money.FromDecimal(decimal.Zero), Currency: "CNY", Status: constants.PaymentStatusPending,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			OrderID: 3, ProviderType: constants.PaymentProviderOfficial, ChannelType: constants.PaymentChannelTypeWechat,
			InteractionMode: constants.PaymentInteractionQR, Amount: money.FromDecimal(decimal.NewFromInt(100)),
			FeeAmount: money.FromDecimal(decimal.NewFromInt(3)), FeePolicy: constants.PaymentFeePolicyMerchantAbsorbed,
			Currency: "CNY", Status: constants.PaymentStatusPending, CreatedAt: now, UpdatedAt: now,
		},
	}
	if err := db.Create(&payments).Error; err != nil {
		t.Fatalf("seed historical payments failed: %v", err)
	}
	if err := ensurePaymentFeePolicyMigration(); err != nil {
		t.Fatalf("migrate payment fee policies failed: %v", err)
	}
	var migrated []paymentdomain.Payment
	if err := db.Order("id asc").Find(&migrated).Error; err != nil {
		t.Fatalf("load migrated payments failed: %v", err)
	}
	if migrated[0].FeePolicy != constants.PaymentFeePolicyLegacyCustomerSurcharge {
		t.Fatalf("fee-bearing history policy = %q", migrated[0].FeePolicy)
	}
	if migrated[1].FeePolicy != constants.PaymentFeePolicyNone {
		t.Fatalf("fee-free history policy = %q", migrated[1].FeePolicy)
	}
	if migrated[2].FeePolicy != constants.PaymentFeePolicyMerchantAbsorbed {
		t.Fatalf("explicit payment policy was overwritten = %q", migrated[2].FeePolicy)
	}

	migrated[0].FeePolicy = constants.PaymentFeePolicyMerchantAbsorbed
	if err := db.Save(&migrated[0]).Error; err != nil {
		t.Fatalf("update post-migration payment failed: %v", err)
	}
	if err := ensurePaymentFeePolicyMigration(); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}
	var unchanged paymentdomain.Payment
	if err := db.First(&unchanged, migrated[0].ID).Error; err != nil {
		t.Fatalf("reload post-migration payment failed: %v", err)
	}
	if unchanged.FeePolicy != constants.PaymentFeePolicyMerchantAbsorbed {
		t.Fatalf("idempotency rewrote payment policy to %q", unchanged.FeePolicy)
	}
}

func TestEnsureOrderRefundPaymentFeeMigrationBackfillsManualRefundsOnce(t *testing.T) {
	dsn := fmt.Sprintf("file:order_refund_payment_fee_migration_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&orderdomain.Order{},
		&orderdomain.OrderRefundRecord{},
		&paymentdomain.Payment{},
		&settingsstore.SettingRecord{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	gormdb.DB = db

	now := time.Now().UTC().Truncate(time.Second)
	order := &orderdomain.Order{
		OrderNo: "REFUND-FEE-MIGRATION-001", Status: constants.OrderStatusRefunded, Currency: "CNY",
		TotalAmount: money.FromDecimal(decimal.NewFromInt(100)), PaidAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	if err := db.Create(&paymentdomain.Payment{
		OrderID: order.ID, ProviderType: constants.PaymentProviderOfficial,
		ChannelType: constants.PaymentChannelTypeAlipay, InteractionMode: constants.PaymentInteractionRedirect,
		Amount: money.FromDecimal(decimal.NewFromInt(100)), FeeAmount: money.FromDecimal(decimal.RequireFromString("3.00")), FeePolicy: constants.PaymentFeePolicyMerchantAbsorbed,
		Currency: "CNY", Status: constants.PaymentStatusSuccess, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}
	manual := &orderdomain.OrderRefundRecord{
		OrderID: order.ID, Type: constants.OrderRefundTypeManual,
		Amount: money.FromDecimal(decimal.NewFromInt(50)), Currency: "CNY", CreatedAt: now, UpdatedAt: now,
	}
	wallet := &orderdomain.OrderRefundRecord{
		OrderID: order.ID, Type: constants.OrderRefundTypeWallet,
		Amount: money.FromDecimal(decimal.NewFromInt(50)), Currency: "CNY", CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
	}
	if err := db.Create(manual).Error; err != nil {
		t.Fatalf("create manual refund failed: %v", err)
	}
	if err := db.Create(wallet).Error; err != nil {
		t.Fatalf("create wallet refund failed: %v", err)
	}

	if err := ensureOrderRefundPaymentFeeMigration(); err != nil {
		t.Fatalf("migrate refund payment fees failed: %v", err)
	}
	if err := db.First(manual, manual.ID).Error; err != nil {
		t.Fatalf("reload manual refund failed: %v", err)
	}
	if !manual.PaymentFeeRefunded || !manual.PaymentFeeRefundedAmount.Decimal.Equal(decimal.RequireFromString("1.50")) {
		t.Fatalf("manual refund was not backfilled: %+v", manual)
	}
	if err := db.First(wallet, wallet.ID).Error; err != nil {
		t.Fatalf("reload wallet refund failed: %v", err)
	}
	if wallet.PaymentFeeRefunded || !wallet.PaymentFeeRefundedAmount.Decimal.IsZero() {
		t.Fatalf("wallet refund should not be backfilled: %+v", wallet)
	}

	if err := db.Model(manual).Updates(map[string]interface{}{
		"payment_fee_refunded":        false,
		"payment_fee_refunded_amount": money.FromDecimal(decimal.Zero),
	}).Error; err != nil {
		t.Fatalf("correct migrated refund failed: %v", err)
	}
	if err := ensureOrderRefundPaymentFeeMigration(); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}
	if err := db.First(manual, manual.ID).Error; err != nil {
		t.Fatalf("reload corrected refund failed: %v", err)
	}
	if manual.PaymentFeeRefunded || !manual.PaymentFeeRefundedAmount.Decimal.IsZero() {
		t.Fatalf("idempotent migration overwrote correction: %+v", manual)
	}
}

func TestEnsurePaymentProviderBepusdtRenameMigration_RenamesAndIsIdempotent(t *testing.T) {
	db := setupPaymentProviderRenameTestDB(t)

	// Seed: 两条 provider_type='epusdt'（旧 BEpusdt）+ 一条无关
	now := time.Now()
	if err := db.Create(&paymentdomain.PaymentChannel{
		Name: "old-bepusdt-1", ProviderType: "epusdt", ChannelType: "usdt-trc20",
		InteractionMode: "redirect", IsActive: true, ConfigJSON: jsonmap.JSON{},
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed channel 1 failed: %v", err)
	}
	if err := db.Create(&paymentdomain.PaymentChannel{
		Name: "old-bepusdt-2", ProviderType: "epusdt", ChannelType: "trx",
		InteractionMode: "redirect", IsActive: true, ConfigJSON: jsonmap.JSON{},
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed channel 2 failed: %v", err)
	}
	if err := db.Create(&paymentdomain.PaymentChannel{
		Name: "alipay", ProviderType: "official", ChannelType: "alipay",
		InteractionMode: "redirect", IsActive: true, ConfigJSON: jsonmap.JSON{},
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed channel 3 failed: %v", err)
	}

	// First run: should rename
	if err := ensurePaymentProviderBepusdtRenameMigration(); err != nil {
		t.Fatalf("first migration failed: %v", err)
	}

	var renamed []paymentdomain.PaymentChannel
	if err := db.Where("provider_type = ?", "bepusdt").Find(&renamed).Error; err != nil {
		t.Fatalf("query bepusdt failed: %v", err)
	}
	if len(renamed) != 2 {
		t.Fatalf("expected 2 bepusdt channels after migration, got %d", len(renamed))
	}

	var stillEpusdt int64
	if err := db.Model(&paymentdomain.PaymentChannel{}).Where("provider_type = ?", "epusdt").Count(&stillEpusdt).Error; err != nil {
		t.Fatalf("count epusdt failed: %v", err)
	}
	if stillEpusdt != 0 {
		t.Fatalf("expected 0 epusdt channels after migration, got %d", stillEpusdt)
	}

	// Marker should be written
	var marker settingsstore.SettingRecord
	if err := db.First(&marker, "key = ?", "migration/payment_provider_bepusdt_rename_v1").Error; err != nil {
		t.Fatalf("expected marker after migration: %v", err)
	}
	if !migrationDone(marker.ValueJSON) {
		t.Fatalf("marker should have done=true, got %v", marker.ValueJSON)
	}

	// Now seed a NEW real epusdt channel (post-migration scenario)
	if err := db.Create(&paymentdomain.PaymentChannel{
		Name: "real-epusdt", ProviderType: "epusdt", ChannelType: "usdt-trc20",
		InteractionMode: "redirect", IsActive: true, ConfigJSON: jsonmap.JSON{},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed real epusdt failed: %v", err)
	}

	// Second run: marker hits, should be a no-op for the new real epusdt
	if err := ensurePaymentProviderBepusdtRenameMigration(); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}

	var realEpusdtCount int64
	if err := db.Model(&paymentdomain.PaymentChannel{}).Where("name = ? AND provider_type = ?", "real-epusdt", "epusdt").Count(&realEpusdtCount).Error; err != nil {
		t.Fatalf("count real epusdt failed: %v", err)
	}
	if realEpusdtCount != 1 {
		t.Fatalf("idempotency broken: real epusdt should still be provider_type='epusdt', count=%d", realEpusdtCount)
	}

	// And bepusdt count should still be 2 (not 3, the new real epusdt didn't get migrated)
	var bepusdtCount int64
	if err := db.Model(&paymentdomain.PaymentChannel{}).Where("provider_type = ?", "bepusdt").Count(&bepusdtCount).Error; err != nil {
		t.Fatalf("count bepusdt failed: %v", err)
	}
	if bepusdtCount != 2 {
		t.Fatalf("idempotency broken: bepusdt count expected 2, got %d", bepusdtCount)
	}
}

func TestEnsurePaymentChannelBepusdtConfigMigration_NormalizesLegacyChannels(t *testing.T) {
	db := setupPaymentProviderRenameTestDB(t)
	tests := []struct {
		name        string
		channelType string
		tradeType   string
	}{
		{name: "legacy-usdt", channelType: "usdt", tradeType: "usdt.trc20"},
		{name: "legacy-usdt-trc20", channelType: "usdt-trc20", tradeType: "usdt.trc20"},
		{name: "legacy-usdc-trc20", channelType: "usdc-trc20", tradeType: "usdt.trc20"},
		{name: "legacy-trx", channelType: "trx", tradeType: "usdt.trc20"},
	}
	for _, tc := range tests {
		if err := db.Create(&paymentdomain.PaymentChannel{
			Name: tc.name, ProviderType: "bepusdt", ChannelType: tc.channelType,
			InteractionMode: "redirect", IsActive: true, ConfigJSON: jsonmap.JSON{"gateway_url": "https://bepusdt.example.com"},
		}).Error; err != nil {
			t.Fatalf("seed %s failed: %v", tc.name, err)
		}
	}
	if err := db.Create(&paymentdomain.PaymentChannel{
		Name: "explicit", ProviderType: "bepusdt", ChannelType: "usdt-trc20",
		InteractionMode: "redirect", IsActive: true, ConfigJSON: jsonmap.JSON{"trade_type": "usdt.arbitrum"},
	}).Error; err != nil {
		t.Fatalf("seed explicit failed: %v", err)
	}
	if err := db.Create(&paymentdomain.PaymentChannel{
		Name: "unknown", ProviderType: "bepusdt", ChannelType: "future-coin",
		InteractionMode: "redirect", IsActive: true, ConfigJSON: jsonmap.JSON{},
	}).Error; err != nil {
		t.Fatalf("seed unknown failed: %v", err)
	}

	if err := ensurePaymentChannelBepusdtConfigMigration(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	for _, tc := range tests {
		var channel paymentdomain.PaymentChannel
		if err := db.First(&channel, "name = ?", tc.name).Error; err != nil {
			t.Fatalf("load %s failed: %v", tc.name, err)
		}
		if channel.ChannelType != "bepusdt" {
			t.Fatalf("%s channel_type = %q, want bepusdt", tc.name, channel.ChannelType)
		}
		if got := channel.ConfigJSON["trade_type"]; got != tc.tradeType {
			t.Fatalf("%s trade_type = %v, want %s", tc.name, got, tc.tradeType)
		}
		if got := channel.ConfigJSON["order_mode"]; got != "transaction" {
			t.Fatalf("%s order_mode = %v, want transaction", tc.name, got)
		}
	}

	var explicit paymentdomain.PaymentChannel
	if err := db.First(&explicit, "name = ?", "explicit").Error; err != nil {
		t.Fatalf("load explicit failed: %v", err)
	}
	if got := explicit.ConfigJSON["trade_type"]; got != "usdt.arbitrum" {
		t.Fatalf("explicit trade_type changed to %v", got)
	}
	if explicit.ChannelType != "bepusdt" {
		t.Fatalf("explicit channel_type = %q, want bepusdt", explicit.ChannelType)
	}
	if got := explicit.ConfigJSON["order_mode"]; got != "transaction" {
		t.Fatalf("explicit order_mode = %v, want transaction", got)
	}
	var unknown paymentdomain.PaymentChannel
	if err := db.First(&unknown, "name = ?", "unknown").Error; err != nil {
		t.Fatalf("load unknown failed: %v", err)
	}
	if unknown.ChannelType != "future-coin" || len(unknown.ConfigJSON) != 0 {
		t.Fatalf("unknown channel should stay unchanged: channel_type=%q config=%v", unknown.ChannelType, unknown.ConfigJSON)
	}

	if err := ensurePaymentChannelBepusdtConfigMigration(); err != nil {
		t.Fatalf("second migration should be idempotent: %v", err)
	}
	var marker settingsstore.SettingRecord
	if err := db.First(&marker, "key = ?", paymentChannelBepusdtConfigMigrationSettingKey).Error; err != nil {
		t.Fatalf("load marker failed: %v", err)
	}
	if !migrationDone(marker.ValueJSON) || marker.ValueJSON["migrated_count"] != float64(len(tests)+1) {
		t.Fatalf("unexpected marker: %v", marker.ValueJSON)
	}
}

package exchange_test

import (
	"fmt"
	"testing"
	"time"

	paymentapp "github.com/dujiao-next/internal/modules/payment/application"

	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletgormstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// --- Integration tests: exchange rate with callback verification ---

func setupExchangeTest(t *testing.T) (*paymentapp.PaymentService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:payment_exchange_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&walletdomain.RechargeOrder{},
		&paymentdomain.PaymentChannel{},
		&paymentdomain.Payment{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	gormdb.DB = db

	orderRepo := ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	productRepo := productgormstore.NewProductStore(db)
	productSKURepo := productgormstore.NewSKUStore(db)
	paymentRepo := paymentgormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	channelRepo := paymentgormstore.NewChannelStore(db)
	walletRepo := walletgormstore.New(db)
	walletSvc := walletapp.NewService(walletapp.Options{
		Repository:   walletRepo,
		Transactions: walletRepo,
	})
	paymentSvc := paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{
		OrderStore:     orderRepo,
		ProductRepo:    productRepo,
		ProductSKURepo: productSKURepo,
		PaymentStore:   paymentRepo,
		ChannelStore:   channelRepo,
		WalletRepo:     walletRepo,
		WalletService:  walletSvc,
		ExpireMinutes:  15,
	})
	return paymentSvc, db
}

func createExchangePaymentFixture(t *testing.T, db *gorm.DB, originalAmount decimal.Decimal, originalCurrency string, convertedAmount decimal.Decimal, convertedCurrency string, exchangeRate string) (*paymentdomain.Payment, *orderdomain.Order) {
	t.Helper()
	now := time.Now()

	user := &userdomain.User{
		Email:        fmt.Sprintf("exchange_test_%d@example.com", now.UnixNano()),
		PasswordHash: "hash",
		Status:       constants.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	order := &orderdomain.Order{
		OrderNo:          fmt.Sprintf("DJEXTEST%d", now.UnixNano()),
		UserID:           user.ID,
		Status:           constants.OrderStatusPendingPayment,
		Currency:         originalCurrency,
		OriginalAmount:   money.FromDecimal(originalAmount),
		TotalAmount:      money.FromDecimal(originalAmount),
		OnlinePaidAmount: money.FromDecimal(decimal.Zero),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	payment := &paymentdomain.Payment{
		OrderID:         order.ID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeAlipay,
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          money.FromDecimal(convertedAmount),
		Currency:        convertedCurrency,
		FeeRate:         money.FromDecimal(decimal.Zero),
		FixedFee:        money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Status:          constants.PaymentStatusPending,
		ProviderRef:     fmt.Sprintf("PAY-%d", now.UnixNano()),
		GatewayOrderNo:  order.OrderNo,
		ProviderPayload: jsonmap.JSON{
			"exchange_rate":     exchangeRate,
			"original_amount":   originalAmount.StringFixed(2),
			"original_currency": originalCurrency,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	return payment, order
}

func TestCallbackMatchesConvertedAmount(t *testing.T) {
	svc, db := setupExchangeTest(t)
	// Order: $10 USD, converted to ¥72 CNY (rate 7.2)
	payment, order := createExchangePaymentFixture(t, db,
		decimal.NewFromInt(10), "USD",
		decimal.RequireFromString("72"), "CNY",
		"7.2",
	)

	now := time.Now()
	updated, err := svc.HandleCallback(paymentapp.PaymentCallbackInput{
		PaymentID:   payment.ID,
		OrderNo:     order.OrderNo,
		ChannelID:   payment.ChannelID,
		Status:      constants.PaymentStatusSuccess,
		ProviderRef: "ALIPAY-SUCCESS-001",
		Amount:      money.FromDecimal(decimal.RequireFromString("72")),
		Currency:    "CNY",
		PaidAt:      &now,
	})
	if err != nil {
		t.Fatalf("callback should succeed with converted amount: %v", err)
	}
	if updated.Status != constants.PaymentStatusSuccess {
		t.Errorf("status = %s, want %s", updated.Status, constants.PaymentStatusSuccess)
	}
}

func TestCallbackRejectsOriginalAmountWhenConverted(t *testing.T) {
	svc, db := setupExchangeTest(t)
	// Payment stored as ¥72 CNY (converted from $10 USD)
	payment, order := createExchangePaymentFixture(t, db,
		decimal.NewFromInt(10), "USD",
		decimal.RequireFromString("72"), "CNY",
		"7.2",
	)

	now := time.Now()
	_, err := svc.HandleCallback(paymentapp.PaymentCallbackInput{
		PaymentID:   payment.ID,
		OrderNo:     order.OrderNo,
		ChannelID:   payment.ChannelID,
		Status:      constants.PaymentStatusSuccess,
		ProviderRef: "ALIPAY-FAIL-001",
		Amount:      money.FromDecimal(decimal.NewFromInt(10)), // original USD amount
		Currency:    "CNY",
		PaidAt:      &now,
	})
	if err != paymentapp.ErrPaymentAmountMismatch {
		t.Fatalf("callback with original amount should fail with paymentapp.ErrPaymentAmountMismatch, got: %v", err)
	}
}

func TestCallbackRejectsCurrencyMismatchAfterConversion(t *testing.T) {
	svc, db := setupExchangeTest(t)
	payment, order := createExchangePaymentFixture(t, db,
		decimal.NewFromInt(10), "USD",
		decimal.RequireFromString("72"), "CNY",
		"7.2",
	)

	now := time.Now()
	_, err := svc.HandleCallback(paymentapp.PaymentCallbackInput{
		PaymentID:   payment.ID,
		OrderNo:     order.OrderNo,
		ChannelID:   payment.ChannelID,
		Status:      constants.PaymentStatusSuccess,
		ProviderRef: "REF-001",
		Amount:      money.FromDecimal(decimal.RequireFromString("72")),
		Currency:    "USD", // wrong currency
		PaidAt:      &now,
	})
	if err != paymentapp.ErrPaymentCurrencyMismatch {
		t.Fatalf("callback with wrong currency should fail with paymentapp.ErrPaymentCurrencyMismatch, got: %v", err)
	}
}

func TestCallbackRejectsMissingAmountForSuccess(t *testing.T) {
	svc, db := setupExchangeTest(t)
	payment, order := createExchangePaymentFixture(t, db,
		decimal.NewFromInt(10), "USD",
		decimal.RequireFromString("72"), "CNY",
		"7.2",
	)

	now := time.Now()
	_, err := svc.HandleCallback(paymentapp.PaymentCallbackInput{
		PaymentID:   payment.ID,
		OrderNo:     order.OrderNo,
		ChannelID:   payment.ChannelID,
		Status:      constants.PaymentStatusSuccess,
		ProviderRef: "REF-002",
		Amount:      money.FromDecimal(decimal.Zero),
		Currency:    "CNY",
		PaidAt:      &now,
	})
	if err != paymentapp.ErrPaymentAmountMismatch {
		t.Fatalf("callback without amount should fail with ErrPaymentAmountMismatch, got: %v", err)
	}
	var stored paymentdomain.Payment
	if err := db.First(&stored, payment.ID).Error; err != nil {
		t.Fatalf("reload payment: %v", err)
	}
	if stored.Status == constants.PaymentStatusSuccess {
		t.Fatalf("missing amount callback must not mark payment successful")
	}
}

func TestCallbackRejectsMissingCurrencyForSuccess(t *testing.T) {
	svc, db := setupExchangeTest(t)
	payment, order := createExchangePaymentFixture(t, db,
		decimal.NewFromInt(10), "USD",
		decimal.RequireFromString("72"), "CNY",
		"7.2",
	)

	now := time.Now()
	_, err := svc.HandleCallback(paymentapp.PaymentCallbackInput{
		PaymentID:   payment.ID,
		OrderNo:     order.OrderNo,
		ChannelID:   payment.ChannelID,
		Status:      constants.PaymentStatusSuccess,
		ProviderRef: "REF-003",
		Amount:      money.FromDecimal(decimal.RequireFromString("72")),
		Currency:    "", // empty = skip check
		PaidAt:      &now,
	})
	if err != paymentapp.ErrPaymentCurrencyMismatch {
		t.Fatalf("callback without currency should fail with ErrPaymentCurrencyMismatch, got: %v", err)
	}
	var stored paymentdomain.Payment
	if err := db.First(&stored, payment.ID).Error; err != nil {
		t.Fatalf("reload payment: %v", err)
	}
	if stored.Status == constants.PaymentStatusSuccess {
		t.Fatalf("missing currency callback must not mark payment successful")
	}
}

func TestCallbackNoConversionPaymentStillWorks(t *testing.T) {
	svc, db := setupExchangeTest(t)
	now := time.Now()

	user := &userdomain.User{
		Email: fmt.Sprintf("noconv_%d@example.com", now.UnixNano()), PasswordHash: "h", Status: constants.UserStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	order := &orderdomain.Order{
		OrderNo: fmt.Sprintf("DJNOCONV%d", now.UnixNano()), UserID: user.ID,
		Status: constants.OrderStatusPendingPayment, Currency: "USD",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(25)),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(25)),
		OnlinePaidAmount: money.FromDecimal(decimal.Zero),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		CreatedAt:        now, UpdatedAt: now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	payment := &paymentdomain.Payment{
		OrderID: order.ID, ChannelID: 1,
		ProviderType: constants.PaymentProviderOfficial, ChannelType: constants.PaymentChannelTypeStripe,
		InteractionMode: constants.PaymentInteractionRedirect,
		Amount:          money.FromDecimal(decimal.NewFromInt(25)), Currency: "USD",
		FeeRate: money.FromDecimal(decimal.Zero), FixedFee: money.FromDecimal(decimal.Zero),
		FeeAmount: money.FromDecimal(decimal.Zero),
		Status:    constants.PaymentStatusPending, ProviderRef: "pi_test",
		GatewayOrderNo: order.OrderNo, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	updated, err := svc.HandleCallback(paymentapp.PaymentCallbackInput{
		PaymentID: payment.ID, OrderNo: order.OrderNo, ChannelID: payment.ChannelID,
		Status: constants.PaymentStatusSuccess, ProviderRef: "pi_test",
		Amount: money.FromDecimal(decimal.NewFromInt(25)), Currency: "USD", PaidAt: &now,
	})
	if err != nil {
		t.Fatalf("callback without conversion should succeed: %v", err)
	}
	if updated.Status != constants.PaymentStatusSuccess {
		t.Errorf("status = %s, want %s", updated.Status, constants.PaymentStatusSuccess)
	}
}

func TestCallbackRejectsAmountMismatchWithoutConversion(t *testing.T) {
	svc, db := setupExchangeTest(t)
	now := time.Now()

	user := &userdomain.User{
		Email: fmt.Sprintf("mismatch_%d@example.com", now.UnixNano()), PasswordHash: "h", Status: constants.UserStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	order := &orderdomain.Order{
		OrderNo: fmt.Sprintf("DJMISMATCH%d", now.UnixNano()), UserID: user.ID,
		Status: constants.OrderStatusPendingPayment, Currency: "USD",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(25)),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(25)),
		OnlinePaidAmount: money.FromDecimal(decimal.Zero),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		CreatedAt:        now, UpdatedAt: now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	payment := &paymentdomain.Payment{
		OrderID: order.ID, ChannelID: 1,
		ProviderType: constants.PaymentProviderOfficial, ChannelType: constants.PaymentChannelTypeStripe,
		InteractionMode: constants.PaymentInteractionRedirect,
		Amount:          money.FromDecimal(decimal.NewFromInt(25)), Currency: "USD",
		FeeRate: money.FromDecimal(decimal.Zero), FixedFee: money.FromDecimal(decimal.Zero),
		FeeAmount: money.FromDecimal(decimal.Zero),
		Status:    constants.PaymentStatusPending, ProviderRef: "pi_test2",
		GatewayOrderNo: order.OrderNo, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	_, err := svc.HandleCallback(paymentapp.PaymentCallbackInput{
		PaymentID: payment.ID, OrderNo: order.OrderNo, ChannelID: payment.ChannelID,
		Status: constants.PaymentStatusSuccess, ProviderRef: "pi_test2",
		Amount: money.FromDecimal(decimal.NewFromInt(10)), Currency: "USD", PaidAt: &now,
	})
	if err != paymentapp.ErrPaymentAmountMismatch {
		t.Fatalf("callback with mismatched amount should fail, got: %v", err)
	}
}

func TestCallbackIdempotentSuccessWithConvertedAmount(t *testing.T) {
	svc, db := setupExchangeTest(t)
	payment, order := createExchangePaymentFixture(t, db,
		decimal.NewFromInt(10), "USD",
		decimal.RequireFromString("72"), "CNY",
		"7.2",
	)
	// Mark payment as already successful
	paidAt := time.Now()
	db.Model(payment).Updates(map[string]interface{}{
		"status":  constants.PaymentStatusSuccess,
		"paid_at": paidAt,
	})

	updated, err := svc.HandleCallback(paymentapp.PaymentCallbackInput{
		PaymentID:   payment.ID,
		OrderNo:     order.OrderNo,
		ChannelID:   payment.ChannelID,
		Status:      constants.PaymentStatusSuccess,
		ProviderRef: "ALIPAY-IDEM-001",
		Amount:      money.FromDecimal(decimal.RequireFromString("72")),
		Currency:    "CNY",
		PaidAt:      &paidAt,
	})
	if err != nil {
		t.Fatalf("idempotent callback should succeed: %v", err)
	}
	if updated.Status != constants.PaymentStatusSuccess {
		t.Errorf("status = %s, want %s", updated.Status, constants.PaymentStatusSuccess)
	}
}

// --- Exchange rate conversion scenarios table test ---

func TestExchangeConversionScenarios(t *testing.T) {
	tests := []struct {
		name              string
		originalAmount    string
		originalCurrency  string
		convertedAmount   string
		convertedCurrency string
		exchangeRate      string
		callbackAmount    string
		callbackCurrency  string
		wantErr           error
	}{
		{
			name:           "alipay USD to CNY success",
			originalAmount: "10.00", originalCurrency: "USD",
			convertedAmount: "72.00", convertedCurrency: "CNY",
			exchangeRate:   "7.2",
			callbackAmount: "72.00", callbackCurrency: "CNY",
			wantErr: nil,
		},
		{
			name:           "wechat USD to CNY success",
			originalAmount: "5.50", originalCurrency: "USD",
			convertedAmount: "39.60", convertedCurrency: "CNY",
			exchangeRate:   "7.2",
			callbackAmount: "39.60", callbackCurrency: "CNY",
			wantErr: nil,
		},
		{
			name:           "stripe USD to EUR success",
			originalAmount: "100.00", originalCurrency: "USD",
			convertedAmount: "92.00", convertedCurrency: "EUR",
			exchangeRate:   "0.92",
			callbackAmount: "92.00", callbackCurrency: "EUR",
			wantErr: nil,
		},
		{
			name:           "epay USD to CNY success",
			originalAmount: "15.00", originalCurrency: "USD",
			convertedAmount: "108.00", convertedCurrency: "CNY",
			exchangeRate:   "7.2",
			callbackAmount: "108.00", callbackCurrency: "CNY",
			wantErr: nil,
		},
		{
			name:           "amount mismatch rejected",
			originalAmount: "10.00", originalCurrency: "USD",
			convertedAmount: "72.00", convertedCurrency: "CNY",
			exchangeRate:   "7.2",
			callbackAmount: "71.00", callbackCurrency: "CNY",
			wantErr: paymentapp.ErrPaymentAmountMismatch,
		},
		{
			name:           "currency mismatch rejected",
			originalAmount: "10.00", originalCurrency: "USD",
			convertedAmount: "72.00", convertedCurrency: "CNY",
			exchangeRate:   "7.2",
			callbackAmount: "72.00", callbackCurrency: "USD",
			wantErr: paymentapp.ErrPaymentCurrencyMismatch,
		},
		{
			name:           "original amount as callback rejected",
			originalAmount: "20.00", originalCurrency: "USD",
			convertedAmount: "144.00", convertedCurrency: "CNY",
			exchangeRate:   "7.2",
			callbackAmount: "20.00", callbackCurrency: "CNY",
			wantErr: paymentapp.ErrPaymentAmountMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, db := setupExchangeTest(t)
			payment, order := createExchangePaymentFixture(t, db,
				decimal.RequireFromString(tt.originalAmount), tt.originalCurrency,
				decimal.RequireFromString(tt.convertedAmount), tt.convertedCurrency,
				tt.exchangeRate,
			)

			now := time.Now()
			_, err := svc.HandleCallback(paymentapp.PaymentCallbackInput{
				PaymentID:   payment.ID,
				OrderNo:     order.OrderNo,
				ChannelID:   payment.ChannelID,
				Status:      constants.PaymentStatusSuccess,
				ProviderRef: fmt.Sprintf("REF-%d", now.UnixNano()),
				Amount:      money.FromDecimal(decimal.RequireFromString(tt.callbackAmount)),
				Currency:    tt.callbackCurrency,
				PaidAt:      &now,
			})

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("want err %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// --- ProviderPayload audit data verification ---

func TestProviderPayloadRetainsOriginalAfterConversion(t *testing.T) {
	svc, db := setupExchangeTest(t)
	payment, order := createExchangePaymentFixture(t, db,
		decimal.NewFromInt(10), "USD",
		decimal.RequireFromString("72"), "CNY",
		"7.2",
	)

	now := time.Now()
	updated, err := svc.HandleCallback(paymentapp.PaymentCallbackInput{
		PaymentID:   payment.ID,
		OrderNo:     order.OrderNo,
		ChannelID:   payment.ChannelID,
		Status:      constants.PaymentStatusSuccess,
		ProviderRef: "AUDIT-001",
		Amount:      money.FromDecimal(decimal.RequireFromString("72")),
		Currency:    "CNY",
		PaidAt:      &now,
	})
	if err != nil {
		t.Fatalf("callback failed: %v", err)
	}

	// Reload from database to verify persisted data
	var dbPayment paymentdomain.Payment
	if err := db.First(&dbPayment, updated.ID).Error; err != nil {
		t.Fatalf("reload payment failed: %v", err)
	}

	if dbPayment.Amount.String() != "72.00" {
		t.Errorf("persisted Amount = %s, want 72.00", dbPayment.Amount.String())
	}
	if dbPayment.Currency != "CNY" {
		t.Errorf("persisted Currency = %s, want CNY", dbPayment.Currency)
	}

	// Verify audit trail in ProviderPayload
	if dbPayment.ProviderPayload == nil {
		t.Fatal("ProviderPayload should not be nil")
	}
	if dbPayment.ProviderPayload["original_amount"] != "10.00" {
		t.Errorf("original_amount = %v, want 10.00", dbPayment.ProviderPayload["original_amount"])
	}
	if dbPayment.ProviderPayload["original_currency"] != "USD" {
		t.Errorf("original_currency = %v, want USD", dbPayment.ProviderPayload["original_currency"])
	}
	if dbPayment.ProviderPayload["exchange_rate"] != "7.2" {
		t.Errorf("exchange_rate = %v, want 7.2", dbPayment.ProviderPayload["exchange_rate"])
	}
}

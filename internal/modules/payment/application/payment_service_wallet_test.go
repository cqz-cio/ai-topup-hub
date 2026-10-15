package application

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	xorpayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/xorpay"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	alipayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/alipay"
	bepusdtadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/bepusdt"
	dujiaopayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/dujiaopay"
	epayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/epay"
	epusdtadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/epusdt"
	okpayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/okpay"
	paypaladapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/paypal"
	stripeadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/stripe"
	tokenpayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/tokenpay"
	wechatpayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/wechatpay"

	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletgormstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"

	"github.com/dujiao-next/internal/constants"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/provider"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupPaymentServiceWalletTest(t *testing.T) (*PaymentService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:payment_service_wallet_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
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
		&walletdomain.OrderFunding{},
		&walletdomain.EarningSettlement{},
		&walletdomain.WithdrawRequest{},
		&walletdomain.RechargeOrder{},
		&paymentdomain.PaymentChannel{},
		&paymentdomain.Payment{},
		&settingsstore.SettingRecord{},
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

	// 构建与 Container.BuildRunner 相同的 PaymentProviderRegistry，
	// 确保 applyProviderPayment 通过 Registry 路由时各 adapter 可以被找到。
	reg := provider.NewRegistry()
	reg.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeStripe, stripeadapter.NewStripeAdapter())
	reg.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypePaypal, paypaladapter.NewPaypalAdapter())
	reg.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeWechat, wechatpayadapter.NewWechatpayAdapter())
	reg.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeAlipay, alipayadapter.NewAlipayAdapter())
	reg.Register(constants.PaymentProviderEpay, "", epayadapter.NewEpayAdapter())
	reg.Register(constants.PaymentProviderEpusdt, "", epusdtadapter.NewEpusdtAdapter())
	reg.Register(constants.PaymentProviderBepusdt, "", bepusdtadapter.NewBepusdtAdapter())
	reg.Register(constants.PaymentProviderDujiaoPay, "", dujiaopayadapter.NewDujiaoPayAdapter())
	reg.Register(constants.PaymentProviderTokenpay, "", tokenpayadapter.NewTokenpayAdapter())
	reg.Register(constants.PaymentProviderOkpay, "", okpayadapter.NewOkpayAdapter())

	paymentSvc := NewPaymentService(PaymentServiceOptions{
		OrderStore:              orderRepo,
		ProductRepo:             productRepo,
		ProductSKURepo:          productSKURepo,
		PaymentStore:            paymentRepo,
		ChannelStore:            channelRepo,
		WalletRepo:              walletRepo,
		UserStore:               userstore.New(db),
		WalletService:           walletSvc,
		SettingService:          settingsapp.NewService(settingsstore.New(db)),
		ExpireMinutes:           15,
		PaymentProviderRegistry: reg,
	})

	return paymentSvc, db
}

func TestWalletChannelListMatchesRechargeAllowlist(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	channels := []paymentdomain.PaymentChannel{
		{Name: "allowed", ProviderType: "bscusdt", ChannelType: "usdt-bep20", IsActive: true, PaymentTypes: jsonslice.Strings{"order", "wallet"}},
		{Name: "excluded", ProviderType: "plasmausdt", ChannelType: "usdt-plasma", IsActive: true, PaymentTypes: jsonslice.Strings{"order", "wallet"}},
	}
	if err := db.Create(&channels).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := settingsstore.New(db).Upsert(constants.SettingKeyWalletConfig, jsonmap.JSON{"recharge_channel_ids": []uint{channels[0].ID}}); err != nil {
		t.Fatal(err)
	}
	user := &userdomain.User{ID: 1}
	available, err := svc.GetAvailableChannels(AvailablePaymentChannelFilter{User: user, PaymentType: constants.PaymentTypeWallet})
	if err != nil || len(available) != 1 || available[0]["id"] != channels[0].ID {
		t.Fatalf("wallet allowlist ignored: %v %v", available, err)
	}
	orders, err := svc.GetAvailableChannels(AvailablePaymentChannelFilter{User: user, PaymentType: constants.PaymentTypeOrder})
	if err != nil || len(orders) != 2 {
		t.Fatalf("wallet setting restricted order channels: %v", err)
	}
}

func TestCreatePaymentWalletFullAmountCreatesPaymentRecord(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	now := time.Now()

	user := &userdomain.User{
		Email:        "wallet_pay_user@example.com",
		PasswordHash: "hash",
		Status:       constants.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	order := &orderdomain.Order{
		OrderNo:                 "DJTESTWALLETPAY001",
		UserID:                  user.ID,
		Status:                  constants.OrderStatusPendingPayment,
		Currency:                "CNY",
		OriginalAmount:          money.FromDecimal(decimal.NewFromInt(50)),
		DiscountAmount:          money.FromDecimal(decimal.Zero),
		PromotionDiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:             money.FromDecimal(decimal.NewFromInt(50)),
		WalletPaidAmount:        money.FromDecimal(decimal.Zero),
		OnlinePaidAmount:        money.FromDecimal(decimal.NewFromInt(50)),
		RefundedAmount:          money.FromDecimal(decimal.Zero),
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	account := &walletdomain.Account{
		UserID:    user.ID,
		Balance:   money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("create wallet account failed: %v", err)
	}

	result, err := svc.CreatePayment(CreatePaymentInput{
		OrderID:    order.ID,
		ChannelID:  0,
		UseBalance: true,
	})
	if err != nil {
		t.Fatalf("create payment failed: %v", err)
	}
	if !result.OrderPaid {
		t.Fatalf("expected order_paid=true")
	}
	if result.Payment != nil {
		t.Fatalf("expected response payment to be nil for wallet full payment")
	}
	if !result.WalletPaidAmount.Decimal.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("wallet_paid_amount want 50 got %s", result.WalletPaidAmount.String())
	}
	if !result.OnlinePayAmount.Decimal.Equal(decimal.Zero) {
		t.Fatalf("online_pay_amount want 0 got %s", result.OnlinePayAmount.String())
	}

	var payment paymentdomain.Payment
	if err := db.Where("order_id = ?", order.ID).First(&payment).Error; err != nil {
		t.Fatalf("wallet payment record not found: %v", err)
	}
	if payment.ProviderType != constants.PaymentProviderWallet {
		t.Fatalf("provider_type want %s got %s", constants.PaymentProviderWallet, payment.ProviderType)
	}
	if payment.ChannelType != constants.PaymentChannelTypeBalance {
		t.Fatalf("channel_type want %s got %s", constants.PaymentChannelTypeBalance, payment.ChannelType)
	}
	if payment.InteractionMode != constants.PaymentInteractionBalance {
		t.Fatalf("interaction_mode want %s got %s", constants.PaymentInteractionBalance, payment.InteractionMode)
	}
	if payment.ChannelID != 0 {
		t.Fatalf("channel_id want 0 got %d", payment.ChannelID)
	}
	if payment.Status != constants.PaymentStatusSuccess {
		t.Fatalf("payment status want %s got %s", constants.PaymentStatusSuccess, payment.Status)
	}
	if !payment.Amount.Decimal.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("payment amount want 50 got %s", payment.Amount.String())
	}
	if payment.PaidAt == nil {
		t.Fatalf("wallet payment should set paid_at")
	}

	var refreshedOrder orderdomain.Order
	if err := db.First(&refreshedOrder, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if refreshedOrder.Status != constants.OrderStatusPaid {
		t.Fatalf("order status want %s got %s", constants.OrderStatusPaid, refreshedOrder.Status)
	}
	if refreshedOrder.PaidAt == nil {
		t.Fatalf("order should set paid_at")
	}

	var refreshedAccount walletdomain.Account
	if err := db.Where("user_id = ?", user.ID).First(&refreshedAccount).Error; err != nil {
		t.Fatalf("reload wallet account failed: %v", err)
	}
	if !refreshedAccount.Balance.Decimal.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("wallet balance want 50 got %s", refreshedAccount.Balance.String())
	}
}

func TestExpireWalletRechargePaymentPendingToExpired(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	payment, recharge := createWalletRechargeFixture(t, db, constants.PaymentStatusPending, constants.WalletRechargeStatusPending)

	updated, err := svc.ExpireWalletRechargePayment(payment.ID)
	if err != nil {
		t.Fatalf("expire wallet recharge payment failed: %v", err)
	}
	if updated == nil {
		t.Fatalf("expected updated payment")
	}
	if updated.Status != constants.PaymentStatusExpired {
		t.Fatalf("payment status want %s got %s", constants.PaymentStatusExpired, updated.Status)
	}
	if updated.ExpiredAt == nil {
		t.Fatalf("expected payment expired_at set")
	}

	var refreshedPayment paymentdomain.Payment
	if err := db.First(&refreshedPayment, payment.ID).Error; err != nil {
		t.Fatalf("reload payment failed: %v", err)
	}
	if refreshedPayment.Status != constants.PaymentStatusExpired {
		t.Fatalf("reloaded payment status want %s got %s", constants.PaymentStatusExpired, refreshedPayment.Status)
	}
	if refreshedPayment.ExpiredAt == nil {
		t.Fatalf("reloaded payment expected expired_at set")
	}

	var refreshedRecharge walletdomain.RechargeOrder
	if err := db.First(&refreshedRecharge, recharge.ID).Error; err != nil {
		t.Fatalf("reload recharge failed: %v", err)
	}
	if refreshedRecharge.Status != constants.WalletRechargeStatusExpired {
		t.Fatalf("recharge status want %s got %s", constants.WalletRechargeStatusExpired, refreshedRecharge.Status)
	}
}

func TestExpireWalletRechargePaymentDoesNotOverrideSuccess(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	payment, recharge := createWalletRechargeFixture(t, db, constants.PaymentStatusSuccess, constants.WalletRechargeStatusSuccess)

	updated, err := svc.ExpireWalletRechargePayment(payment.ID)
	if err != nil {
		t.Fatalf("expire wallet recharge payment failed: %v", err)
	}
	if updated == nil {
		t.Fatalf("expected updated payment")
	}
	if updated.Status != constants.PaymentStatusSuccess {
		t.Fatalf("payment status want %s got %s", constants.PaymentStatusSuccess, updated.Status)
	}

	var refreshedPayment paymentdomain.Payment
	if err := db.First(&refreshedPayment, payment.ID).Error; err != nil {
		t.Fatalf("reload payment failed: %v", err)
	}
	if refreshedPayment.Status != constants.PaymentStatusSuccess {
		t.Fatalf("reloaded payment status want %s got %s", constants.PaymentStatusSuccess, refreshedPayment.Status)
	}
	if refreshedPayment.PaidAt == nil {
		t.Fatalf("success payment should keep paid_at")
	}

	var refreshedRecharge walletdomain.RechargeOrder
	if err := db.First(&refreshedRecharge, recharge.ID).Error; err != nil {
		t.Fatalf("reload recharge failed: %v", err)
	}
	if refreshedRecharge.Status != constants.WalletRechargeStatusSuccess {
		t.Fatalf("recharge status want %s got %s", constants.WalletRechargeStatusSuccess, refreshedRecharge.Status)
	}
}

func TestExpireWalletRechargePaymentSkipsOrderPayment(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	now := time.Now()
	payment := &paymentdomain.Payment{
		OrderID:         99,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeWechat,
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          money.FromDecimal(decimal.NewFromInt(100)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	updated, err := svc.ExpireWalletRechargePayment(payment.ID)
	if err != nil {
		t.Fatalf("expire wallet recharge payment failed: %v", err)
	}
	if updated == nil {
		t.Fatalf("expected payment result")
	}
	if updated.Status != constants.PaymentStatusPending {
		t.Fatalf("order payment should remain pending, got %s", updated.Status)
	}
}

func TestWalletRechargeCallbackSuccessAfterExpireCreditsOnce(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	payment, recharge := createWalletRechargeFixture(t, db, constants.PaymentStatusPending, constants.WalletRechargeStatusPending)

	if _, err := svc.ExpireWalletRechargePayment(payment.ID); err != nil {
		t.Fatalf("expire wallet recharge payment failed: %v", err)
	}

	updated, err := svc.HandleCallback(buildWalletRechargeCallbackInput(payment, recharge, constants.PaymentStatusSuccess, "CALLBACK-SUCCESS-1"))
	if err != nil {
		t.Fatalf("handle wallet recharge callback failed: %v", err)
	}
	if updated.Status != constants.PaymentStatusSuccess {
		t.Fatalf("payment status want %s got %s", constants.PaymentStatusSuccess, updated.Status)
	}

	assertWalletRechargeSuccessState(t, db, payment.ID, recharge.ID, recharge.UserID, recharge.Amount.Decimal)
}

func TestWalletRechargeCallbackSuccessThenExpireKeepsSuccess(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	payment, recharge := createWalletRechargeFixture(t, db, constants.PaymentStatusPending, constants.WalletRechargeStatusPending)

	if _, err := svc.HandleCallback(buildWalletRechargeCallbackInput(payment, recharge, constants.PaymentStatusSuccess, "CALLBACK-SUCCESS-2")); err != nil {
		t.Fatalf("handle wallet recharge callback failed: %v", err)
	}

	updated, err := svc.ExpireWalletRechargePayment(payment.ID)
	if err != nil {
		t.Fatalf("expire wallet recharge payment failed: %v", err)
	}
	if updated.Status != constants.PaymentStatusSuccess {
		t.Fatalf("payment status want %s got %s", constants.PaymentStatusSuccess, updated.Status)
	}

	assertWalletRechargeSuccessState(t, db, payment.ID, recharge.ID, recharge.UserID, recharge.Amount.Decimal)
}

func TestWalletRechargeCallbackDuplicateSuccessDoesNotDuplicateCredit(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	payment, recharge := createWalletRechargeFixture(t, db, constants.PaymentStatusPending, constants.WalletRechargeStatusPending)

	if _, err := svc.HandleCallback(buildWalletRechargeCallbackInput(payment, recharge, constants.PaymentStatusSuccess, "CALLBACK-SUCCESS-FIRST")); err != nil {
		t.Fatalf("handle first wallet recharge callback failed: %v", err)
	}
	if _, err := svc.HandleCallback(buildWalletRechargeCallbackInput(payment, recharge, constants.PaymentStatusSuccess, "CALLBACK-SUCCESS-SECOND")); err != nil {
		t.Fatalf("handle second wallet recharge callback failed: %v", err)
	}

	assertWalletRechargeSuccessState(t, db, payment.ID, recharge.ID, recharge.UserID, recharge.Amount.Decimal)

	reference := fmt.Sprintf("recharge:%d:success", recharge.ID)
	var txnCount int64
	if err := db.Model(&walletdomain.Transaction{}).Where("reference = ?", reference).Count(&txnCount).Error; err != nil {
		t.Fatalf("count wallet transaction failed: %v", err)
	}
	if txnCount != 1 {
		t.Fatalf("wallet recharge success transaction count want 1 got %d", txnCount)
	}
}

func TestWalletRechargeCallbackConcurrentSuccessCreditsOnce(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	payment, recharge := createWalletRechargeFixture(t, db, constants.PaymentStatusPending, constants.WalletRechargeStatusPending)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get raw db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	const callbacks = 2
	var wait sync.WaitGroup
	errorsCh := make(chan error, callbacks)
	for index := 0; index < callbacks; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			input := buildWalletRechargeCallbackInput(
				payment,
				recharge,
				constants.PaymentStatusSuccess,
				fmt.Sprintf("CALLBACK-CONCURRENT-%d", index),
			)
			if _, callbackErr := svc.HandleCallback(input); callbackErr != nil {
				errorsCh <- callbackErr
			}
		}(index)
	}
	wait.Wait()
	close(errorsCh)
	for callbackErr := range errorsCh {
		t.Errorf("concurrent callback failed: %v", callbackErr)
	}

	assertWalletRechargeSuccessState(t, db, payment.ID, recharge.ID, recharge.UserID, recharge.Amount.Decimal)
	reference := fmt.Sprintf("recharge:%d:success", recharge.ID)
	var transactionCount int64
	if err := db.Model(&walletdomain.Transaction{}).Where("reference = ?", reference).Count(&transactionCount).Error; err != nil {
		t.Fatalf("count wallet transaction: %v", err)
	}
	if transactionCount != 1 {
		t.Fatalf("concurrent success callbacks created %d credits, want 1", transactionCount)
	}
}

func TestWalletRechargeCallbackPendingAfterExpireDoesNotReopen(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	payment, recharge := createWalletRechargeFixture(t, db, constants.PaymentStatusPending, constants.WalletRechargeStatusPending)

	if _, err := svc.ExpireWalletRechargePayment(payment.ID); err != nil {
		t.Fatalf("expire wallet recharge payment failed: %v", err)
	}
	updated, err := svc.HandleCallback(buildWalletRechargeCallbackInput(payment, recharge, constants.PaymentStatusPending, "CALLBACK-PENDING-LATE"))
	if err != nil {
		t.Fatalf("handle wallet recharge callback failed: %v", err)
	}
	if updated.Status != constants.PaymentStatusExpired {
		t.Fatalf("payment status want %s got %s", constants.PaymentStatusExpired, updated.Status)
	}

	var refreshedRecharge walletdomain.RechargeOrder
	if err := db.First(&refreshedRecharge, recharge.ID).Error; err != nil {
		t.Fatalf("reload recharge failed: %v", err)
	}
	if refreshedRecharge.Status != constants.WalletRechargeStatusExpired {
		t.Fatalf("recharge status want %s got %s", constants.WalletRechargeStatusExpired, refreshedRecharge.Status)
	}
}

func TestWalletRechargeCallbackTerminalStateMatrixDoesNotReopen(t *testing.T) {
	cases := []struct {
		name              string
		paymentStatus     string
		rechargeStatus    string
		callbackStatus    string
		wantPaymentStatus string
		wantRechargeState string
	}{
		{
			name:              "expired_to_pending",
			paymentStatus:     constants.PaymentStatusExpired,
			rechargeStatus:    constants.WalletRechargeStatusExpired,
			callbackStatus:    constants.PaymentStatusPending,
			wantPaymentStatus: constants.PaymentStatusExpired,
			wantRechargeState: constants.WalletRechargeStatusExpired,
		},
		{
			name:              "failed_to_pending",
			paymentStatus:     constants.PaymentStatusFailed,
			rechargeStatus:    constants.WalletRechargeStatusFailed,
			callbackStatus:    constants.PaymentStatusPending,
			wantPaymentStatus: constants.PaymentStatusFailed,
			wantRechargeState: constants.WalletRechargeStatusFailed,
		},
		{
			name:              "failed_to_expired",
			paymentStatus:     constants.PaymentStatusFailed,
			rechargeStatus:    constants.WalletRechargeStatusFailed,
			callbackStatus:    constants.PaymentStatusExpired,
			wantPaymentStatus: constants.PaymentStatusFailed,
			wantRechargeState: constants.WalletRechargeStatusFailed,
		},
		{
			name:              "expired_to_failed",
			paymentStatus:     constants.PaymentStatusExpired,
			rechargeStatus:    constants.WalletRechargeStatusExpired,
			callbackStatus:    constants.PaymentStatusFailed,
			wantPaymentStatus: constants.PaymentStatusExpired,
			wantRechargeState: constants.WalletRechargeStatusExpired,
		},
		{
			name:              "success_to_pending",
			paymentStatus:     constants.PaymentStatusSuccess,
			rechargeStatus:    constants.WalletRechargeStatusSuccess,
			callbackStatus:    constants.PaymentStatusPending,
			wantPaymentStatus: constants.PaymentStatusSuccess,
			wantRechargeState: constants.WalletRechargeStatusSuccess,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := setupPaymentServiceWalletTest(t)
			payment, recharge := createWalletRechargeFixture(t, db, tc.paymentStatus, tc.rechargeStatus)

			providerRef := fmt.Sprintf("CALLBACK-%s-%d", tc.name, time.Now().UnixNano())
			updated, err := svc.HandleCallback(buildWalletRechargeCallbackInput(payment, recharge, tc.callbackStatus, providerRef))
			if err != nil {
				t.Fatalf("handle wallet recharge callback failed: %v", err)
			}
			if updated.Status != tc.wantPaymentStatus {
				t.Fatalf("payment status want %s got %s", tc.wantPaymentStatus, updated.Status)
			}

			var refreshedPayment paymentdomain.Payment
			if err := db.First(&refreshedPayment, payment.ID).Error; err != nil {
				t.Fatalf("reload payment failed: %v", err)
			}
			if refreshedPayment.Status != tc.wantPaymentStatus {
				t.Fatalf("reloaded payment status want %s got %s", tc.wantPaymentStatus, refreshedPayment.Status)
			}

			var refreshedRecharge walletdomain.RechargeOrder
			if err := db.First(&refreshedRecharge, recharge.ID).Error; err != nil {
				t.Fatalf("reload recharge failed: %v", err)
			}
			if refreshedRecharge.Status != tc.wantRechargeState {
				t.Fatalf("recharge status want %s got %s", tc.wantRechargeState, refreshedRecharge.Status)
			}
		})
	}
}

func TestWalletRechargeCallbackSuccessAfterFailedCreditsOnce(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	payment, recharge := createWalletRechargeFixture(t, db, constants.PaymentStatusFailed, constants.WalletRechargeStatusFailed)

	updated, err := svc.HandleCallback(buildWalletRechargeCallbackInput(payment, recharge, constants.PaymentStatusSuccess, "CALLBACK-SUCCESS-AFTER-FAILED"))
	if err != nil {
		t.Fatalf("handle wallet recharge callback failed: %v", err)
	}
	if updated.Status != constants.PaymentStatusSuccess {
		t.Fatalf("payment status want %s got %s", constants.PaymentStatusSuccess, updated.Status)
	}

	assertWalletRechargeSuccessState(t, db, payment.ID, recharge.ID, recharge.UserID, recharge.Amount.Decimal)

	reference := fmt.Sprintf("recharge:%d:success", recharge.ID)
	var txnCount int64
	if err := db.Model(&walletdomain.Transaction{}).Where("reference = ?", reference).Count(&txnCount).Error; err != nil {
		t.Fatalf("count wallet transaction failed: %v", err)
	}
	if txnCount != 1 {
		t.Fatalf("wallet recharge success transaction count want 1 got %d", txnCount)
	}
}

type failWalletRechargeReloadRepository struct {
	walletcontract.Repository
	getCalls int
}

func (r *failWalletRechargeReloadRepository) GetRechargeOrderByPaymentID(paymentID uint) (*walletdomain.RechargeOrder, error) {
	r.getCalls++
	if r.getCalls > 1 {
		return nil, fmt.Errorf("forced recharge reload failure")
	}
	return r.Repository.GetRechargeOrderByPaymentID(paymentID)
}

type recordingMemberLevelProgressor struct {
	rechargeUserID uint
	rechargeAmount decimal.Decimal
}

func (r *recordingMemberLevelProgressor) OnOrderPaid(uint, decimal.Decimal) error {
	return nil
}

func (r *recordingMemberLevelProgressor) OnRechargeCompleted(userID uint, amount decimal.Decimal) error {
	r.rechargeUserID = userID
	r.rechargeAmount = amount
	return nil
}

func TestWalletRechargeCallbackReloadFailureKeepsNotificationContext(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	payment, recharge := createWalletRechargeFixture(t, db, constants.PaymentStatusPending, constants.WalletRechargeStatusPending)
	svc.walletRepo = &failWalletRechargeReloadRepository{Repository: svc.walletRepo}
	progressor := &recordingMemberLevelProgressor{}
	svc.SetMemberLevelService(progressor)

	if _, err := svc.HandleCallback(buildWalletRechargeCallbackInput(payment, recharge, constants.PaymentStatusSuccess, "CALLBACK-RELOAD-FAILURE")); err != nil {
		t.Fatalf("HandleCallback failed: %v", err)
	}
	if progressor.rechargeUserID != recharge.UserID {
		t.Fatalf("fallback recharge user id = %d, want %d", progressor.rechargeUserID, recharge.UserID)
	}
	if !progressor.rechargeAmount.Equal(recharge.Amount.Decimal) {
		t.Fatalf("fallback recharge amount = %s, want %s", progressor.rechargeAmount, recharge.Amount.String())
	}
}

func TestHandleDujiaoPayWebhookAppliesVerifiedWalletRecharge(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	channel, payment, recharge := createDujiaoPayWalletWebhookFixture(t, db)

	body := []byte(fmt.Sprintf(
		`{"event_id":"evt-wallet-paid","event_type":"order.paid","event_version":"v1","created_at":"2026-07-27T00:00:00Z","data":{"order_id":%q,"merchant_order_id":%q,"fiat_currency":"CNY","fiat_amount":"88.00","tx_id":"0xwallet","paid_at":"2026-07-27T00:00:01Z"}}`,
		payment.ProviderRef,
		recharge.RechargeNo,
	))
	headers := signDujiaoPayWebhook(body, "test-dujiaopay-webhook-secret")

	updated, status, err := svc.HandleDujiaoPayWebhook(WebhookCallbackInput{
		ChannelID: channel.ID,
		Headers:   headers,
		Body:      body,
		Context:   context.Background(),
	})
	if err != nil {
		t.Fatalf("HandleDujiaoPayWebhook failed: %v", err)
	}
	if status != constants.PaymentStatusSuccess || updated == nil || updated.Status != constants.PaymentStatusSuccess {
		t.Fatalf("webhook result status=%q payment=%+v", status, updated)
	}
	assertWalletRechargeSuccessState(t, db, payment.ID, recharge.ID, recharge.UserID, recharge.Amount.Decimal)
}

func TestHandleDujiaoPayWebhookMarksOrderPaid(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	channel := createDujiaoPayChannelFixture(t, db)
	now := time.Now()
	order := &orderdomain.Order{
		OrderNo:          fmt.Sprintf("DJ-DUJIAOPAY-%d", now.UnixNano()),
		UserID:           1,
		Status:           constants.OrderStatusPendingPayment,
		Currency:         "CNY",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(88)),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(88)),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(88)),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	payment := &paymentdomain.Payment{
		OrderID:         order.ID,
		ChannelID:       channel.ID,
		ProviderType:    constants.PaymentProviderDujiaoPay,
		ChannelType:     channel.ChannelType,
		InteractionMode: constants.PaymentInteractionRedirect,
		Amount:          money.FromDecimal(decimal.NewFromInt(88)),
		Currency:        "CNY",
		Status:          constants.PaymentStatusPending,
		GatewayOrderNo:  fmt.Sprintf("PAY-DUJIAOPAY-%d", now.UnixNano()),
		ProviderRef:     fmt.Sprintf("do_order_%d", now.UnixNano()),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	body := []byte(fmt.Sprintf(
		`{"event_id":"evt-order-paid","event_type":"order.paid","event_version":"v1","created_at":"2026-07-27T00:00:00Z","data":{"order_id":%q,"merchant_order_id":%q,"fiat_currency":"CNY","fiat_amount":"88.00","tx_id":"0xorder"}}`,
		payment.ProviderRef,
		payment.GatewayOrderNo,
	))
	updated, status, err := svc.HandleDujiaoPayWebhook(WebhookCallbackInput{
		ChannelID: channel.ID,
		Headers:   signDujiaoPayWebhook(body, "test-dujiaopay-webhook-secret"),
		Body:      body,
		Context:   context.Background(),
	})
	if err != nil {
		t.Fatalf("HandleDujiaoPayWebhook failed: %v", err)
	}
	if status != constants.PaymentStatusSuccess || updated == nil || updated.Status != constants.PaymentStatusSuccess {
		t.Fatalf("webhook result status=%q payment=%+v", status, updated)
	}
	var storedOrder orderdomain.Order
	if err := db.First(&storedOrder, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if storedOrder.Status != constants.OrderStatusPaid || storedOrder.PaidAt == nil {
		t.Fatalf("DujiaoPay webhook did not mark order paid: %+v", storedOrder)
	}
}

func TestHandleDujiaoPayWebhookRejectsMismatchedMerchantOrderID(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	channel, payment, recharge := createDujiaoPayWalletWebhookFixture(t, db)

	body := []byte(fmt.Sprintf(
		`{"event_id":"evt-wallet-mismatch","event_type":"order.paid","event_version":"v1","created_at":"2026-07-27T00:00:00Z","data":{"order_id":%q,"merchant_order_id":"WRONG-MERCHANT-ORDER","fiat_currency":"CNY","fiat_amount":"88.00","tx_id":"0xmismatch"}}`,
		payment.ProviderRef,
	))
	headers := signDujiaoPayWebhook(body, "test-dujiaopay-webhook-secret")

	if _, _, err := svc.HandleDujiaoPayWebhook(WebhookCallbackInput{
		ChannelID: channel.ID,
		Headers:   headers,
		Body:      body,
		Context:   context.Background(),
	}); err != ErrPaymentInvalid {
		t.Fatalf("mismatched merchant order error = %v, want ErrPaymentInvalid", err)
	}

	var storedRecharge walletdomain.RechargeOrder
	if err := db.First(&storedRecharge, recharge.ID).Error; err != nil {
		t.Fatalf("reload recharge failed: %v", err)
	}
	if storedRecharge.Status != constants.WalletRechargeStatusPending {
		t.Fatalf("mismatched webhook changed recharge status to %s", storedRecharge.Status)
	}
}

func TestHandleDujiaoPayWebhookAdoptsSignedCurrencyForLegacyPendingPayment(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	channel, payment, recharge := createDujiaoPayWalletWebhookFixture(t, db)
	channel.ConfigJSON["fiat_currency"] = "USD"
	if err := db.Model(channel).Update("config_json", channel.ConfigJSON).Error; err != nil {
		t.Fatalf("update channel fiat currency: %v", err)
	}

	body := []byte(fmt.Sprintf(
		`{"event_id":"evt-wallet-legacy-currency","event_type":"order.paid","event_version":"v1","created_at":"2026-07-27T00:00:00Z","data":{"order_id":%q,"merchant_order_id":%q,"fiat_currency":"USD","fiat_amount":"88.00","tx_id":"0xlegacycurrency"}}`,
		payment.ProviderRef,
		recharge.RechargeNo,
	))
	updated, status, err := svc.HandleDujiaoPayWebhook(WebhookCallbackInput{
		ChannelID: channel.ID,
		Headers:   signDujiaoPayWebhook(body, "test-dujiaopay-webhook-secret"),
		Body:      body,
		Context:   context.Background(),
	})
	if err != nil {
		t.Fatalf("legacy DujiaoPay webhook failed: %v", err)
	}
	if status != constants.PaymentStatusSuccess || updated == nil || updated.Currency != "USD" {
		t.Fatalf("legacy webhook status=%q payment=%+v", status, updated)
	}

	var stored paymentdomain.Payment
	if err := db.First(&stored, payment.ID).Error; err != nil {
		t.Fatalf("reload payment: %v", err)
	}
	if stored.Currency != "USD" {
		t.Fatalf("stored currency = %q, want signed legacy currency USD", stored.Currency)
	}
	if got := stored.ProviderPayload[paymentcontract.GatewayPayloadFiatCurrencySent]; got != "USD" {
		t.Fatalf("stored fiat currency snapshot = %#v, want USD", got)
	}
}

func TestHandleDujiaoPayWebhookKeepsStrictCurrencyCheckForSnapshottedPayment(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	channel, payment, recharge := createDujiaoPayWalletWebhookFixture(t, db)
	payment.ProviderPayload = jsonmap.JSON{
		paymentcontract.GatewayPayloadFiatCurrencySent: "CNY",
	}
	if err := db.Model(payment).Update("provider_payload", payment.ProviderPayload).Error; err != nil {
		t.Fatalf("store payment fiat currency snapshot: %v", err)
	}

	body := []byte(fmt.Sprintf(
		`{"event_id":"evt-wallet-strict-currency","event_type":"order.paid","event_version":"v1","created_at":"2026-07-27T00:00:00Z","data":{"order_id":%q,"merchant_order_id":%q,"fiat_currency":"USD","fiat_amount":"88.00","tx_id":"0xstrictcurrency"}}`,
		payment.ProviderRef,
		recharge.RechargeNo,
	))
	if _, _, err := svc.HandleDujiaoPayWebhook(WebhookCallbackInput{
		ChannelID: channel.ID,
		Headers:   signDujiaoPayWebhook(body, "test-dujiaopay-webhook-secret"),
		Body:      body,
		Context:   context.Background(),
	}); err != ErrPaymentCurrencyMismatch {
		t.Fatalf("snapshotted currency mismatch error = %v, want ErrPaymentCurrencyMismatch", err)
	}

	var stored paymentdomain.Payment
	if err := db.First(&stored, payment.ID).Error; err != nil {
		t.Fatalf("reload payment: %v", err)
	}
	if stored.Status != constants.PaymentStatusPending || stored.Currency != "CNY" {
		t.Fatalf("strict mismatch changed payment: %+v", stored)
	}
}

func createDujiaoPayWalletWebhookFixture(t *testing.T, db *gorm.DB) (*paymentdomain.PaymentChannel, *paymentdomain.Payment, *walletdomain.RechargeOrder) {
	t.Helper()
	channel := createDujiaoPayChannelFixture(t, db)
	now := time.Now()
	payment, recharge := createWalletRechargeFixture(t, db, constants.PaymentStatusPending, constants.WalletRechargeStatusPending)
	providerRef := fmt.Sprintf("do_wallet_%d", now.UnixNano())
	if err := db.Model(payment).Updates(map[string]interface{}{
		"channel_id":       channel.ID,
		"provider_type":    constants.PaymentProviderDujiaoPay,
		"channel_type":     channel.ChannelType,
		"interaction_mode": constants.PaymentInteractionRedirect,
		"gateway_order_no": recharge.RechargeNo,
		"provider_ref":     providerRef,
	}).Error; err != nil {
		t.Fatalf("update DujiaoPay payment fixture failed: %v", err)
	}
	payment.ChannelID = channel.ID
	payment.ProviderType = constants.PaymentProviderDujiaoPay
	payment.ChannelType = channel.ChannelType
	payment.InteractionMode = constants.PaymentInteractionRedirect
	payment.GatewayOrderNo = recharge.RechargeNo
	payment.ProviderRef = providerRef
	return channel, payment, recharge
}

func createDujiaoPayChannelFixture(t *testing.T, db *gorm.DB) *paymentdomain.PaymentChannel {
	t.Helper()
	now := time.Now()
	channel := &paymentdomain.PaymentChannel{
		Name:            "DujiaoPay test",
		ProviderType:    constants.PaymentProviderDujiaoPay,
		ChannelType:     "tron-usdt",
		InteractionMode: constants.PaymentInteractionRedirect,
		ConfigJSON: jsonmap.JSON{
			"api_base_url":   "https://pay.example.test",
			"api_key_id":     "test-key",
			"api_secret":     "test-api-secret",
			"webhook_secret": "test-dujiaopay-webhook-secret",
			"order_mode":     constants.PaymentDujiaoPayOrderModeTransaction,
			"chain":          "tron",
			"token_id":       "tron-usdt",
			"fiat_currency":  "CNY",
		},
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("create DujiaoPay channel failed: %v", err)
	}
	return channel
}

func signDujiaoPayWebhook(body []byte, secret string) map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(body)
	return map[string]string{
		"DJP-Webhook-Timestamp": timestamp,
		"DJP-Webhook-Signature": hex.EncodeToString(mac.Sum(nil)),
	}
}

func createWalletRechargeFixture(t *testing.T, db *gorm.DB, paymentStatus string, rechargeStatus string) (*paymentdomain.Payment, *walletdomain.RechargeOrder) {
	t.Helper()
	now := time.Now()
	payment := &paymentdomain.Payment{
		OrderID:         0,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeWechat,
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          money.FromDecimal(decimal.NewFromInt(88)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          paymentStatus,
		ProviderRef:     fmt.Sprintf("RECHARGE-PAY-%d", now.UnixNano()),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if paymentStatus == constants.PaymentStatusSuccess {
		payment.PaidAt = &now
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	recharge := &walletdomain.RechargeOrder{
		RechargeNo:      fmt.Sprintf("WRTEST%d", now.UnixNano()),
		UserID:          1,
		PaymentID:       payment.ID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeWechat,
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          money.FromDecimal(decimal.NewFromInt(88)),
		PayableAmount:   money.FromDecimal(decimal.NewFromInt(88)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          rechargeStatus,
		Remark:          "test",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if rechargeStatus == constants.WalletRechargeStatusSuccess {
		recharge.PaidAt = &now
	}
	if err := db.Create(recharge).Error; err != nil {
		t.Fatalf("create recharge failed: %v", err)
	}
	return payment, recharge
}

func buildWalletRechargeCallbackInput(payment *paymentdomain.Payment, recharge *walletdomain.RechargeOrder, status string, providerRef string) PaymentCallbackInput {
	return PaymentCallbackInput{
		PaymentID:   payment.ID,
		OrderNo:     recharge.RechargeNo,
		ChannelID:   payment.ChannelID,
		Status:      status,
		ProviderRef: providerRef,
		Amount:      payment.Amount,
		Currency:    payment.Currency,
		PaidAt:      ptrTime(time.Now()),
		Payload: jsonmap.JSON{
			"provider_ref": providerRef,
			"status":       status,
		},
	}
}

func assertWalletRechargeSuccessState(t *testing.T, db *gorm.DB, paymentID uint, rechargeID uint, userID uint, rechargeAmount decimal.Decimal) {
	t.Helper()

	var refreshedPayment paymentdomain.Payment
	if err := db.First(&refreshedPayment, paymentID).Error; err != nil {
		t.Fatalf("reload payment failed: %v", err)
	}
	if refreshedPayment.Status != constants.PaymentStatusSuccess {
		t.Fatalf("payment status want %s got %s", constants.PaymentStatusSuccess, refreshedPayment.Status)
	}
	if refreshedPayment.PaidAt == nil {
		t.Fatalf("payment should set paid_at")
	}

	var refreshedRecharge walletdomain.RechargeOrder
	if err := db.First(&refreshedRecharge, rechargeID).Error; err != nil {
		t.Fatalf("reload recharge failed: %v", err)
	}
	if refreshedRecharge.Status != constants.WalletRechargeStatusSuccess {
		t.Fatalf("recharge status want %s got %s", constants.WalletRechargeStatusSuccess, refreshedRecharge.Status)
	}
	if refreshedRecharge.PaidAt == nil {
		t.Fatalf("recharge should set paid_at")
	}

	var account walletdomain.Account
	if err := db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		t.Fatalf("reload wallet account failed: %v", err)
	}
	if !account.Balance.Decimal.Equal(rechargeAmount) {
		t.Fatalf("wallet balance want %s got %s", rechargeAmount.String(), account.Balance.String())
	}
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

func createUnderpaidChannel(t *testing.T, db *gorm.DB, svc *PaymentService, name, channelType string) *paymentdomain.PaymentChannel {
	t.Helper()
	now := time.Now()
	channel := &paymentdomain.PaymentChannel{
		Name: name, ProviderType: constants.PaymentProviderOfficial, ChannelType: channelType,
		InteractionMode: constants.PaymentInteractionQR,
		FeeRate:         money.FromDecimal(decimal.Zero), FixedFee: money.FromDecimal(decimal.Zero),
		ConfigJSON: jsonmap.JSON{}, IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("create payment channel failed: %v", err)
	}
	registerTestGateway(t, svc, channel.ProviderType, channel.ChannelType, emptyProviderRefProvider{})
	return channel
}

func createUnderpaidOrder(t *testing.T, db *gorm.DB, orderNo string, userID uint, total int64) *orderdomain.Order {
	t.Helper()
	now := time.Now()
	order := &orderdomain.Order{
		OrderNo: orderNo, UserID: userID, Status: constants.OrderStatusPendingPayment, Currency: "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(total)), TotalAmount: money.FromDecimal(decimal.NewFromInt(total)),
		DiscountAmount: money.FromDecimal(decimal.Zero), PromotionDiscountAmount: money.FromDecimal(decimal.Zero),
		WalletPaidAmount: money.FromDecimal(decimal.Zero), OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(total)),
		RefundedAmount: money.FromDecimal(decimal.Zero), CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	return order
}

func walletBalance(t *testing.T, db *gorm.DB, userID uint) string {
	t.Helper()
	var account walletdomain.Account
	if err := db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		t.Fatalf("load wallet account failed: %v", err)
	}
	return account.Balance.StringFixed(2)
}

// 余额 5 + 渠道 A 在线 10 混合支付后切到渠道 B，余额被退回、在线应付额抬到 15。
// 此时旧的 A 链接在网关侧仍可支付，只付 10 不得履约整单。
func TestSupersededPaymentUnderpaidDoesNotFulfillOrder(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	now := time.Now()

	user := &userdomain.User{
		Email: "underpaid@example.com", PasswordHash: "hash", Status: constants.UserStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	if err := db.Create(&walletdomain.Account{
		UserID: user.ID, Balance: money.FromDecimal(decimal.NewFromInt(5)), CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create wallet account failed: %v", err)
	}

	channelA := createUnderpaidChannel(t, db, svc, "Underpaid Gateway A", constants.PaymentChannelTypeWechat)
	channelB := createUnderpaidChannel(t, db, svc, "Underpaid Gateway B", constants.PaymentChannelTypeAlipay)
	order := createUnderpaidOrder(t, db, "DJUNDERPAID001", user.ID, 15)

	resultA, err := svc.CreatePayment(CreatePaymentInput{
		OrderID: order.ID, ChannelID: channelA.ID, UseBalance: true, Context: context.Background(),
	})
	if err != nil {
		t.Fatalf("create payment on channel A failed: %v", err)
	}
	if got := resultA.Payment.Amount.StringFixed(2); got != "10.00" {
		t.Fatalf("channel A payment amount want 10.00 got %s", got)
	}

	if _, err := svc.CreatePayment(CreatePaymentInput{
		OrderID: order.ID, ChannelID: channelB.ID, UseBalance: false, Context: context.Background(),
	}); err != nil {
		t.Fatalf("switch to channel B failed: %v", err)
	}
	if got := walletBalance(t, db, user.ID); got != "5.00" {
		t.Fatalf("balance want 5.00 after switching to online payment got %s", got)
	}

	var supersededA paymentdomain.Payment
	if err := db.First(&supersededA, resultA.Payment.ID).Error; err != nil {
		t.Fatalf("reload channel A payment failed: %v", err)
	}
	if supersededA.SupersededAt == nil {
		t.Fatalf("channel A payment should be superseded: %+v", supersededA)
	}

	paid, err := svc.HandleCallback(PaymentCallbackInput{
		PaymentID: supersededA.ID, OrderNo: order.OrderNo, ChannelID: channelA.ID,
		Status: constants.PaymentStatusSuccess, Amount: supersededA.Amount, Currency: "CNY", ProviderRef: "underpaid-ref",
	})
	if err != nil {
		t.Fatalf("handle underpaid callback failed: %v", err)
	}
	if paid.Status != constants.PaymentStatusSuccess {
		t.Fatalf("payment status want success got %s", paid.Status)
	}
	if paid.ExceptionCode != constants.PaymentExceptionUnderpaidSucceeded {
		t.Fatalf("exception code want %s got %s", constants.PaymentExceptionUnderpaidSucceeded, paid.ExceptionCode)
	}

	var reloadedOrder orderdomain.Order
	if err := db.First(&reloadedOrder, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if reloadedOrder.Status != constants.OrderStatusPendingPayment || reloadedOrder.PaidAt != nil {
		t.Fatalf("underpaid callback must not fulfill the order: %+v", reloadedOrder)
	}

	// 用户实付的 10 元转入余额，加上退回的 5 元刚好可以补齐这一单。
	if got := walletBalance(t, db, user.ID); got != "15.00" {
		t.Fatalf("balance want 15.00 after underpaid credit got %s", got)
	}
	var credits []walletdomain.Transaction
	if err := db.Where("user_id = ? AND type = ?", user.ID, constants.WalletTxnTypeOrderUnderpaidCredit).Find(&credits).Error; err != nil {
		t.Fatalf("load underpaid credit transactions failed: %v", err)
	}
	if len(credits) != 1 || credits[0].Amount.StringFixed(2) != "10.00" {
		t.Fatalf("underpaid credit transactions unexpected: %+v", credits)
	}

	// 重复回调不得重复入账。
	if _, err := svc.HandleCallback(PaymentCallbackInput{
		PaymentID: supersededA.ID, OrderNo: order.OrderNo, ChannelID: channelA.ID,
		Status: constants.PaymentStatusSuccess, Amount: supersededA.Amount, Currency: "CNY", ProviderRef: "underpaid-ref",
	}); err != nil {
		t.Fatalf("replay underpaid callback failed: %v", err)
	}
	if got := walletBalance(t, db, user.ID); got != "15.00" {
		t.Fatalf("balance want 15.00 after replayed callback got %s", got)
	}

	// 余额补齐后用户可以正常完成订单。
	settled, err := svc.CreatePayment(CreatePaymentInput{
		OrderID: order.ID, UseBalance: true, Context: context.Background(),
	})
	if err != nil {
		t.Fatalf("settle order with wallet balance failed: %v", err)
	}
	if !settled.OrderPaid {
		t.Fatalf("order should be paid by wallet balance: %+v", settled)
	}
	if got := walletBalance(t, db, user.ID); got != "0.00" {
		t.Fatalf("balance want 0.00 after settling the order got %s", got)
	}
}

// 游客订单没有钱包账户：同样不得履约，只留异常码等待人工处理。
func TestUnderpaidGuestOrderIsNotFulfilled(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	channel := createUnderpaidChannel(t, db, svc, "Guest Underpaid Gateway", constants.PaymentChannelTypeWechat)
	order := createUnderpaidOrder(t, db, "DJUNDERPAID002", 0, 15)

	now := time.Now()
	stale := &paymentdomain.Payment{
		OrderID: order.ID, ChannelID: channel.ID, ProviderType: channel.ProviderType, ChannelType: channel.ChannelType,
		InteractionMode: channel.InteractionMode, Amount: money.FromDecimal(decimal.NewFromInt(10)),
		FeeAmount: money.FromDecimal(decimal.Zero), FeePolicy: constants.PaymentFeePolicyNone,
		Currency: "CNY", Status: constants.PaymentStatusPending, QRCode: "stale-link",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(stale).Error; err != nil {
		t.Fatalf("create stale payment failed: %v", err)
	}

	paid, err := svc.HandleCallback(PaymentCallbackInput{
		PaymentID: stale.ID, OrderNo: order.OrderNo, ChannelID: channel.ID,
		Status: constants.PaymentStatusSuccess, Amount: stale.Amount, Currency: "CNY", ProviderRef: "guest-underpaid-ref",
	})
	if err != nil {
		t.Fatalf("handle guest underpaid callback failed: %v", err)
	}
	if paid.ExceptionCode != constants.PaymentExceptionUnderpaidSucceeded {
		t.Fatalf("exception code want %s got %s", constants.PaymentExceptionUnderpaidSucceeded, paid.ExceptionCode)
	}

	var reloadedOrder orderdomain.Order
	if err := db.First(&reloadedOrder, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if reloadedOrder.Status != constants.OrderStatusPendingPayment || reloadedOrder.PaidAt != nil {
		t.Fatalf("guest underpaid callback must not fulfill the order: %+v", reloadedOrder)
	}
}

// 足额支付不受影响：金额守恒校验只拦欠额，不改变正常履约路径。
func TestFullyPaidCallbackStillFulfillsOrder(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	channel := createUnderpaidChannel(t, db, svc, "Full Amount Gateway", constants.PaymentChannelTypeWechat)
	order := createUnderpaidOrder(t, db, "DJUNDERPAID003", 0, 15)

	created, err := svc.CreatePayment(CreatePaymentInput{
		OrderID: order.ID, ChannelID: channel.ID, Context: context.Background(),
	})
	if err != nil {
		t.Fatalf("create payment failed: %v", err)
	}
	paid, err := svc.HandleCallback(PaymentCallbackInput{
		PaymentID: created.Payment.ID, OrderNo: order.OrderNo, ChannelID: channel.ID,
		Status: constants.PaymentStatusSuccess, Amount: created.Payment.Amount, Currency: "CNY", ProviderRef: "full-ref",
	})
	if err != nil {
		t.Fatalf("handle full amount callback failed: %v", err)
	}
	if paid.ExceptionCode != "" {
		t.Fatalf("full amount payment should not be flagged, got %s", paid.ExceptionCode)
	}

	var reloadedOrder orderdomain.Order
	if err := db.First(&reloadedOrder, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if reloadedOrder.Status != constants.OrderStatusPaid || reloadedOrder.PaidAt == nil {
		t.Fatalf("full amount payment should fulfill the order: %+v", reloadedOrder)
	}
}

// 余额分配在"用余额 → 改在线 → 再用余额"之间来回切换时，每一轮都必须真实扣款。
// 轮次幂等键缺失时第二轮会命中上一轮早已退回的流水，订单被标记为已用余额、钱包却没扣钱。
func TestWalletBalanceReappliedAfterReleaseDebitsAgain(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	now := time.Now()

	user := &userdomain.User{
		Email: "reapply@example.com", PasswordHash: "hash", Status: constants.UserStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	if err := db.Create(&walletdomain.Account{
		UserID: user.ID, Balance: money.FromDecimal(decimal.NewFromInt(5)), CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create wallet account failed: %v", err)
	}

	channelA := createUnderpaidChannel(t, db, svc, "Reapply Gateway A", constants.PaymentChannelTypeWechat)
	channelB := createUnderpaidChannel(t, db, svc, "Reapply Gateway B", constants.PaymentChannelTypeAlipay)
	order := createUnderpaidOrder(t, db, "DJUNDERPAID004", user.ID, 15)

	if _, err := svc.CreatePayment(CreatePaymentInput{
		OrderID: order.ID, ChannelID: channelA.ID, UseBalance: true, Context: context.Background(),
	}); err != nil {
		t.Fatalf("first wallet allocation failed: %v", err)
	}
	if got := walletBalance(t, db, user.ID); got != "0.00" {
		t.Fatalf("balance want 0.00 after first allocation got %s", got)
	}

	if _, err := svc.CreatePayment(CreatePaymentInput{
		OrderID: order.ID, ChannelID: channelB.ID, UseBalance: false, Context: context.Background(),
	}); err != nil {
		t.Fatalf("switch to online payment failed: %v", err)
	}
	if got := walletBalance(t, db, user.ID); got != "5.00" {
		t.Fatalf("balance want 5.00 after release got %s", got)
	}

	result, err := svc.CreatePayment(CreatePaymentInput{
		OrderID: order.ID, ChannelID: channelA.ID, UseBalance: true, Context: context.Background(),
	})
	if err != nil {
		t.Fatalf("second wallet allocation failed: %v", err)
	}
	if got := walletBalance(t, db, user.ID); got != "0.00" {
		t.Fatalf("balance want 0.00 after re-applying the balance got %s", got)
	}
	if got := result.WalletPaidAmount.StringFixed(2); got != "5.00" {
		t.Fatalf("wallet_paid_amount want 5.00 got %s", got)
	}
	if got := result.Payment.Amount.StringFixed(2); got != "10.00" {
		t.Fatalf("online payment amount want 10.00 got %s", got)
	}

	var reloadedOrder orderdomain.Order
	if err := db.First(&reloadedOrder, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if got := reloadedOrder.WalletPaidAmount.StringFixed(2); got != "5.00" {
		t.Fatalf("order wallet_paid_amount want 5.00 got %s", got)
	}
}

func TestPaymentCoveredOrderAmount(t *testing.T) {
	tests := []struct {
		name      string
		amount    string
		feeAmount string
		feePolicy string
		want      string
	}{
		{name: "no fee", amount: "15.00", feeAmount: "0", feePolicy: constants.PaymentFeePolicyNone, want: "15"},
		{name: "merchant absorbed", amount: "100.00", feeAmount: "3.00", feePolicy: constants.PaymentFeePolicyMerchantAbsorbed, want: "100"},
		{name: "customer surcharge", amount: "103.00", feeAmount: "3.00", feePolicy: constants.PaymentFeePolicyCustomerSurcharge, want: "100"},
		{name: "legacy customer surcharge", amount: "103.00", feeAmount: "3.00", feePolicy: constants.PaymentFeePolicyLegacyCustomerSurcharge, want: "100"},
		{name: "missing policy snapshot with fee", amount: "103.00", feeAmount: "3.00", feePolicy: "", want: "100"},
		{name: "missing policy snapshot without fee", amount: "100.00", feeAmount: "0", feePolicy: "", want: "100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payment := &paymentdomain.Payment{
				Amount:    money.FromDecimal(decimal.RequireFromString(tt.amount)),
				FeeAmount: money.FromDecimal(decimal.RequireFromString(tt.feeAmount)),
				FeePolicy: tt.feePolicy,
			}
			if got := paymentCoveredOrderAmount(payment, decimal.Zero); got.String() != tt.want {
				t.Fatalf("paymentCoveredOrderAmount() = %s, want %s", got.String(), tt.want)
			}
		})
	}
}

// TestPaymentCoveredOrderAmountWithExchangeRateFullPayment 验证换汇渠道（如 Stripe
// 配置 target_currency=GBP、exchange_rate=0.11，订单 1 CNY 换算成 0.11 GBP 下单）
// 足额支付时，按渠道汇率把回调金额折算回订单币种，能正确判定覆盖了订单应付额，
// 不会拿 GBP 数值直接跟 CNY 总额硬比而误判为金额不足。
func TestPaymentCoveredOrderAmountWithExchangeRateFullPayment(t *testing.T) {
	payment := &paymentdomain.Payment{
		Amount:    money.FromDecimal(decimal.RequireFromString("0.11")),
		FeeAmount: money.FromDecimal(decimal.Zero),
		FeePolicy: constants.PaymentFeePolicyNone,
		Currency:  "GBP",
		ProviderPayload: jsonmap.JSON{
			"original_amount":   "1",
			"original_currency": "CNY",
			"exchange_rate":     "0.11",
		},
	}
	callbackAmount := decimal.RequireFromString("0.11")
	got := paymentCoveredOrderAmount(payment, callbackAmount)
	want := decimal.RequireFromString("1")
	if !got.Equal(want) {
		t.Fatalf("paymentCoveredOrderAmount() = %s, want %s", got.String(), want.String())
	}
}

// TestStripeExchangeRateFullPaymentFulfillsOrder 验证 Stripe 渠道配置了
// target_currency=GBP、exchange_rate=0.11（订单 1 CNY 换算成 0.11 GBP 下单），webhook
// 回调返回 GBP 原值 0.11 时，应按汇率折算回 1 CNY 判定为足额支付，而不是把 0.11 直接
// 当 CNY 数值比较误判为欠款。
func TestStripeExchangeRateFullPaymentFulfillsOrder(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	channel := createUnderpaidChannel(t, db, svc, "Stripe GBP", constants.PaymentChannelTypeStripe)
	order := createUnderpaidOrder(t, db, "STRIPE_GBP_FULL", 1, 1)

	now := time.Now()
	payment := &paymentdomain.Payment{
		OrderID: order.ID, ChannelID: channel.ID, ProviderType: channel.ProviderType, ChannelType: channel.ChannelType,
		InteractionMode: channel.InteractionMode,
		Amount:          money.FromDecimal(decimal.RequireFromString("0.11")),
		FeeAmount:       money.FromDecimal(decimal.Zero), FeePolicy: constants.PaymentFeePolicyNone,
		Currency: "GBP",
		Status:   constants.PaymentStatusPending,
		ProviderPayload: jsonmap.JSON{
			"original_amount":   "1",
			"original_currency": "CNY",
			"exchange_rate":     "0.11",
		},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	paid, err := svc.HandleCallback(PaymentCallbackInput{
		PaymentID: payment.ID, OrderNo: order.OrderNo, ChannelID: channel.ID,
		Status: constants.PaymentStatusSuccess,
		Amount: money.FromDecimal(decimal.RequireFromString("0.11")), Currency: "GBP",
		ProviderRef: "stripe-gbp-full-ref",
	})
	if err != nil {
		t.Fatalf("handle stripe gbp full payment callback failed: %v", err)
	}
	if paid.ExceptionCode != "" {
		t.Fatalf("stripe gbp full payment should not be flagged, got exception_code=%s", paid.ExceptionCode)
	}

	var reloadedOrder orderdomain.Order
	if err := db.First(&reloadedOrder, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if reloadedOrder.Status != constants.OrderStatusPaid || reloadedOrder.PaidAt == nil {
		t.Fatalf("stripe gbp full payment must fulfill the order, got status=%s paid_at=%v", reloadedOrder.Status, reloadedOrder.PaidAt)
	}
}

// TestStripeExchangeRateUnderpaidCallbackRejected 验证 Stripe GBP 换汇场景（订单 1
// CNY，应收 0.11 GBP，实际回调只有 0.05 GBP）被 validateCallbackPaymentFacts 精确
// 匹配直接拒绝——法币网关同样不允许少付，回调金额必须精确等于创建时的 payment.Amount。
func TestStripeExchangeRateUnderpaidCallbackRejected(t *testing.T) {
	svc, db := setupPaymentServiceWalletTest(t)
	channel := createUnderpaidChannel(t, db, svc, "Stripe GBP Underpaid", constants.PaymentChannelTypeStripe)
	order := createUnderpaidOrder(t, db, "STRIPE_GBP_UNDERPAID", 1, 1)

	now := time.Now()
	payment := &paymentdomain.Payment{
		OrderID: order.ID, ChannelID: channel.ID, ProviderType: channel.ProviderType, ChannelType: channel.ChannelType,
		InteractionMode: channel.InteractionMode,
		Amount:          money.FromDecimal(decimal.RequireFromString("0.11")),
		FeeAmount:       money.FromDecimal(decimal.Zero), FeePolicy: constants.PaymentFeePolicyNone,
		Currency: "GBP",
		Status:   constants.PaymentStatusPending,
		ProviderPayload: jsonmap.JSON{
			"original_amount":   "1",
			"original_currency": "CNY",
			"exchange_rate":     "0.11",
		},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	_, err := svc.HandleCallback(PaymentCallbackInput{
		PaymentID: payment.ID, OrderNo: order.OrderNo, ChannelID: channel.ID,
		Status: constants.PaymentStatusSuccess,
		Amount: money.FromDecimal(decimal.RequireFromString("0.05")), Currency: "GBP",
		ProviderRef: "stripe-gbp-underpaid-ref",
	})
	if err != ErrPaymentAmountMismatch {
		t.Fatalf("stripe gbp underpaid callback should be rejected with ErrPaymentAmountMismatch, got: %v", err)
	}

	var reloadedOrder orderdomain.Order
	if err := db.First(&reloadedOrder, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if reloadedOrder.Status != constants.OrderStatusPendingPayment || reloadedOrder.PaidAt != nil {
		t.Fatalf("rejected stripe gbp payment must not fulfill order, got status=%s paid_at=%v", reloadedOrder.Status, reloadedOrder.PaidAt)
	}
}

// Mock only the outbound create call; retain the real XorPay verifier.
type xorpayFixtureGateway struct {
	paymentcontract.GatewayCallbackVerifier
}

func (g xorpayFixtureGateway) CreatePayment(_ context.Context, _ jsonmap.JSON, in paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	return &paymentcontract.GatewayCreateResult{ProviderRef: "ao-" + in.OrderNo, QRCodeURL: "weixin://wxpay/bizpayurl?pr=mock"}, nil
}
func xorpayFixture(t *testing.T) (*PaymentService, *gorm.DB, *paymentdomain.PaymentChannel, *userdomain.User) {
	t.Helper()
	svc, db := setupPaymentServiceWalletTest(t)
	registerTestGateway(t, svc, constants.PaymentProviderXorpay, "", xorpayFixtureGateway{xorpayadapter.NewXorpayAdapter().(paymentcontract.GatewayCallbackVerifier)})
	user := &userdomain.User{Email: "xorpay@example.com", PasswordHash: "fixture", Status: constants.UserStatusActive}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	channel := &paymentdomain.PaymentChannel{FeeRate: money.FromDecimal(decimal.RequireFromString("1.60")), Name: "微信支付", ProviderType: constants.PaymentProviderXorpay, ChannelType: "wechat", InteractionMode: "qr", IsActive: true, PaymentTypes: jsonslice.Strings{"order", "wallet"}, ConfigJSON: jsonmap.JSON{"aid": "12345", "app_secret": "fixture-secret", "customer_fee_enabled": true, "notify_url": "https://shop.example/api/v1/payments/callback"}}
	if err := svc.ValidateChannel(channel); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	return svc, db, channel, user
}
func xorpayForm(payment *paymentdomain.Payment) url.Values {
	f := url.Values{"aoid": {payment.ProviderRef}, "order_id": {payment.GatewayOrderNo}, "pay_price": {payment.Amount.String()}, "pay_time": {time.Now().In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01-02 15:04:05")}}
	signXorpayForm(f)
	return f
}
func signXorpayForm(f url.Values) {
	f.Set("sign", fmt.Sprintf("%x", md5.Sum([]byte(f.Get("aoid")+f.Get("order_id")+f.Get("pay_price")+f.Get("pay_time")+"fixture-secret"))))
}
func xorpayOrder(t *testing.T, db *gorm.DB, userID uint) *orderdomain.Order {
	t.Helper()
	amount := money.FromDecimal(decimal.RequireFromString("12.30"))
	deadline := time.Now().Add(15 * time.Minute)
	o := &orderdomain.Order{OrderNo: "DJ-XORPAY-TEST", UserID: userID, Status: constants.OrderStatusPendingPayment, Currency: "CNY", OriginalAmount: amount, TotalAmount: amount, OnlinePaidAmount: amount, ExpiresAt: &deadline}
	if err := db.Create(o).Error; err != nil {
		t.Fatal(err)
	}
	return o
}

func TestXorpayOrderCallbackFactsAndIdempotency(t *testing.T) {
	for _, scenario := range []string{"success", "bad-sign", "wrong-amount", "wrong-aoid", "wrong-order", "duplicate-field"} {
		t.Run(scenario, func(t *testing.T) {
			svc, db, ch, user := xorpayFixture(t)
			order := xorpayOrder(t, db, user.ID)
			created, err := svc.CreatePayment(CreatePaymentInput{OrderID: order.ID, ChannelID: ch.ID, Context: context.Background()})
			if err != nil {
				t.Fatal(err)
			}
			f := xorpayForm(created.Payment)
			switch scenario {
			case "bad-sign":
				f.Set("sign", "forged")
			case "wrong-amount":
				f.Set("pay_price", "0.01")
				signXorpayForm(f)
			case "wrong-aoid":
				f.Set("aoid", "another-payment")
				signXorpayForm(f)
			case "wrong-order":
				f.Set("order_id", "another-order")
				signXorpayForm(f)
			case "duplicate-field":
				f.Add("order_id", created.Payment.GatewayOrderNo)
			}
			_, err = svc.HandleSyncCallback(ch, f, nil)
			if scenario == "success" {
				if err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 3; i++ {
					if _, err := svc.HandleSyncCallback(ch, f, nil); err != nil {
						t.Fatal(err)
					}
				}
			} else if err == nil {
				t.Fatal("invalid callback accepted")
			}
			var saved orderdomain.Order
			db.First(&saved, order.ID)
			if scenario == "success" {
				if saved.Status != constants.OrderStatusPaid || saved.OnlinePaidAmount.String() != "12.30" {
					t.Fatalf("order not settled exactly once: %+v", saved)
				}
			} else if saved.Status != constants.OrderStatusPendingPayment || saved.PaidAt != nil {
				t.Fatal("invalid callback changed order")
			}
		})
	}
}

func TestXorpayWalletCallbackCreditsExactlyOnce(t *testing.T) {
	svc, db, ch, user := xorpayFixture(t)
	result, err := svc.CreateWalletRechargePayment(CreateWalletRechargePaymentInput{UserID: user.ID, ChannelID: ch.ID, Amount: money.FromDecimal(decimal.RequireFromString("12.30")), Currency: "CNY", Context: context.Background()})
	if err != nil {
		t.Fatal(err)
	}
	f := xorpayForm(result.Payment)
	bad := xorpayForm(result.Payment)
	bad.Set("pay_price", "0.01")
	signXorpayForm(bad)
	if _, err := svc.HandleSyncCallback(ch, bad, nil); err == nil {
		t.Fatal("underpaid recharge accepted")
	}
	for i := 0; i < 4; i++ {
		if _, err := svc.HandleSyncCallback(ch, f, nil); err != nil {
			t.Fatal(err)
		}
	}
	if balance := walletBalance(t, db, user.ID); balance != "12.30" {
		t.Fatalf("wrong balance %s", balance)
	}
	var count int64
	db.Model(&walletdomain.Transaction{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate wallet credit: %d", count)
	}
}

func TestXorpayExpiredOrderDoesNotFulfill(t *testing.T) {
	svc, db, ch, user := xorpayFixture(t)
	order := xorpayOrder(t, db, user.ID)
	result, err := svc.CreatePayment(CreatePaymentInput{OrderID: order.ID, ChannelID: ch.ID, Context: context.Background()})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(order).Update("status", constants.OrderStatusCanceled).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.HandleSyncCallback(ch, xorpayForm(result.Payment), nil); err != nil {
		t.Fatal(err)
	}
	var saved orderdomain.Order
	db.First(&saved, order.ID)
	if saved.Status != constants.OrderStatusCanceled {
		t.Fatal("late payment fulfilled canceled order")
	}
	var payment paymentdomain.Payment
	db.First(&payment, result.Payment.ID)
	if payment.ExceptionCode == "" {
		t.Fatal("late payment not flagged for review")
	}
}

func TestXorpayRejectsRedirectConfiguration(t *testing.T) {
	svc, _, ch, _ := xorpayFixture(t)
	ch.InteractionMode = "redirect"
	if svc.ValidateChannel(ch) == nil {
		t.Fatal("unsupported interaction allowed")
	}
}

func TestXorpayFeeRoundingAndChannelIsolation(t *testing.T) {
	svc, _, ch, _ := xorpayFixture(t)
	for _, tc := range []struct{ base, fee, total string }{{"99.00", "1.59", "100.59"}, {"100.00", "1.60", "101.60"}, {"1160.00", "18.56", "1178.56"}, {"0.01", "0.01", "0.02"}, {"12.30", "0.20", "12.50"}} {
		total, fee, policy := calculateChannelPaymentAmounts(decimal.RequireFromString(tc.base), ch, false)
		if total.StringFixed(2) != tc.total || fee.StringFixed(2) != tc.fee || policy != constants.PaymentFeePolicyCustomerSurcharge {
			t.Fatalf("wrong fee for %s: %s %s %s", tc.base, total, fee, policy)
		}
	}
	channels, err := svc.GetAvailableChannels(AvailablePaymentChannelFilter{PaymentType: constants.PaymentTypeOrder})
	if err != nil || len(channels) != 1 || channels[0]["fee_rounding"] != "ceil" || channels[0]["fee_policy"] != constants.PaymentFeePolicyCustomerSurcharge {
		t.Fatalf("public fee preview mismatch: %v %v", channels, err)
	}
	ch.ConfigJSON["customer_fee_enabled"] = false
	total, _, policy := calculateChannelPaymentAmounts(decimal.NewFromInt(99), ch, true)
	if total.StringFixed(2) != "99.00" || policy != constants.PaymentFeePolicyMerchantAbsorbed {
		t.Fatal("explicit merchant fee not honored")
	}
	ch.ProviderType = constants.PaymentProviderEpay
	total, fee, policy := calculateChannelPaymentAmounts(decimal.NewFromInt(99), ch, false)
	if total.StringFixed(2) != "99.00" || fee.StringFixed(2) != "1.58" || policy != constants.PaymentFeePolicyMerchantAbsorbed {
		t.Fatal("other provider fee policy changed")
	}
}

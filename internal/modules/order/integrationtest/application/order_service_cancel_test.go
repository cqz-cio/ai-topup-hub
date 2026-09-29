package application_test

import (
	"fmt"
	"testing"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	. "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	"github.com/dujiao-next/internal/constants"
	coupongormstore "github.com/dujiao-next/internal/modules/coupon/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestCancelExpiredOrderExpiresPendingPayments(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_expire_payments_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
		&cardsecretdomain.Batch{},
		&cardsecretdomain.Secret{},
		&paymentdomain.Payment{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	expiresAt := now.Add(-time.Hour)
	order := &orderdomain.Order{
		OrderNo:          "EXPIRE-PAYMENT-001",
		Status:           constants.OrderStatusPendingPayment,
		Currency:         "CNY",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount:   money.FromDecimal(decimal.Zero),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(100)),
		RefundedAmount:   money.FromDecimal(decimal.Zero),
		ExpiresAt:        &expiresAt,
		CreatedAt:        now.Add(-2 * time.Hour),
		UpdatedAt:        now.Add(-2 * time.Hour),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	pendingPayment := &paymentdomain.Payment{
		OrderID:         order.ID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderEpay,
		ChannelType:     "alipay",
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          money.FromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		Status:          constants.PaymentStatusPending,
		CreatedAt:       now.Add(-2 * time.Hour),
		UpdatedAt:       now.Add(-2 * time.Hour),
	}
	initiatedPayment := &paymentdomain.Payment{
		OrderID:         order.ID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderEpay,
		ChannelType:     "alipay",
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          money.FromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		Status:          constants.PaymentStatusInitiated,
		CreatedAt:       now.Add(-2 * time.Hour),
		UpdatedAt:       now.Add(-2 * time.Hour),
	}
	successPayment := &paymentdomain.Payment{
		OrderID:         order.ID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderEpay,
		ChannelType:     "alipay",
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          money.FromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		Status:          constants.PaymentStatusSuccess,
		CreatedAt:       now.Add(-2 * time.Hour),
		UpdatedAt:       now.Add(-2 * time.Hour),
		PaidAt:          &now,
	}
	if err := db.Create([]*paymentdomain.Payment{pendingPayment, initiatedPayment, successPayment}).Error; err != nil {
		t.Fatalf("create payments failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		OrderStore:       ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
		ProductStore:     productgormstore.NewProductStore(db),
		ProductSKUStore:  productgormstore.NewSKUStore(db),
		CouponStore:      coupongormstore.New(db),
		CouponUsageStore: coupongormstore.NewUsageStore(db),
	})
	updated, err := svc.CancelExpiredOrder(order.ID)
	if err != nil {
		t.Fatalf("cancel expired order failed: %v", err)
	}
	if updated == nil || updated.Status != constants.OrderStatusCanceled {
		t.Fatalf("expected canceled order, got: %+v", updated)
	}

	var reloaded []paymentdomain.Payment
	if err := db.Order("id asc").Find(&reloaded, "order_id = ?", order.ID).Error; err != nil {
		t.Fatalf("reload payments failed: %v", err)
	}
	if len(reloaded) != 3 {
		t.Fatalf("expected 3 payments, got %d", len(reloaded))
	}
	if reloaded[0].Status != constants.PaymentStatusExpired || reloaded[0].ExpiredAt == nil {
		t.Fatalf("pending payment should expire, got status=%s expired_at=%v", reloaded[0].Status, reloaded[0].ExpiredAt)
	}
	if reloaded[1].Status != constants.PaymentStatusExpired || reloaded[1].ExpiredAt == nil {
		t.Fatalf("initiated payment should expire, got status=%s expired_at=%v", reloaded[1].Status, reloaded[1].ExpiredAt)
	}
	if reloaded[2].Status != constants.PaymentStatusSuccess || reloaded[2].ExpiredAt != nil {
		t.Fatalf("success payment should stay success without expired_at, got status=%s expired_at=%v", reloaded[2].Status, reloaded[2].ExpiredAt)
	}
}

func setupCancelPaymentTestDB(t *testing.T, namespace string) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:order_service_%s_%d?mode=memory&cache=shared", namespace, time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
		&cardsecretdomain.Batch{},
		&cardsecretdomain.Secret{},
		&paymentdomain.Payment{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}

func newPendingOrderForCancel(orderNo string, userID uint, parentID *uint, now time.Time) *orderdomain.Order {
	return &orderdomain.Order{
		OrderNo:          orderNo,
		UserID:           userID,
		ParentID:         parentID,
		Status:           constants.OrderStatusPendingPayment,
		Currency:         "CNY",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount:   money.FromDecimal(decimal.Zero),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(100)),
		RefundedAmount:   money.FromDecimal(decimal.Zero),
		CreatedAt:        now.Add(-2 * time.Hour),
		UpdatedAt:        now.Add(-2 * time.Hour),
	}
}

func newPaymentForOrder(orderID uint, status string, now time.Time) *paymentdomain.Payment {
	return &paymentdomain.Payment{
		OrderID:         orderID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderEpay,
		ChannelType:     "alipay",
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          money.FromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		Status:          status,
		CreatedAt:       now.Add(-2 * time.Hour),
		UpdatedAt:       now.Add(-2 * time.Hour),
	}
}

func TestCancelOrderExpiresPendingPayments(t *testing.T) {
	db := setupCancelPaymentTestDB(t, "user_cancel_expire_payments")

	now := time.Now()
	const userID uint = 42
	order := newPendingOrderForCancel("USER-CANCEL-001", userID, nil, now)
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	pending := newPaymentForOrder(order.ID, constants.PaymentStatusPending, now)
	initiated := newPaymentForOrder(order.ID, constants.PaymentStatusInitiated, now)
	if err := db.Create([]*paymentdomain.Payment{pending, initiated}).Error; err != nil {
		t.Fatalf("create payments failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		OrderStore:       ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
		ProductStore:     productgormstore.NewProductStore(db),
		ProductSKUStore:  productgormstore.NewSKUStore(db),
		CouponStore:      coupongormstore.New(db),
		CouponUsageStore: coupongormstore.NewUsageStore(db),
	})
	updated, err := svc.CancelOrder(order.ID, userID)
	if err != nil {
		t.Fatalf("cancel order failed: %v", err)
	}
	if updated == nil || updated.Status != constants.OrderStatusCanceled {
		t.Fatalf("expected canceled order, got: %+v", updated)
	}

	var reloaded []paymentdomain.Payment
	if err := db.Order("id asc").Find(&reloaded, "order_id = ?", order.ID).Error; err != nil {
		t.Fatalf("reload payments failed: %v", err)
	}
	if len(reloaded) != 2 {
		t.Fatalf("expected 2 payments, got %d", len(reloaded))
	}
	for _, p := range reloaded {
		if p.Status != constants.PaymentStatusExpired || p.ExpiredAt == nil {
			t.Fatalf("payment %d should expire, got status=%s expired_at=%v", p.ID, p.Status, p.ExpiredAt)
		}
	}
}

func TestUpdateOrderStatusAdminCancelExpiresPendingPaymentsSingleOrder(t *testing.T) {
	db := setupCancelPaymentTestDB(t, "admin_cancel_expire_payments_single")

	now := time.Now()
	order := newPendingOrderForCancel("ADMIN-CANCEL-SINGLE-001", 0, nil, now)
	order.GuestEmail = "buyer@example.com"
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	pending := newPaymentForOrder(order.ID, constants.PaymentStatusPending, now)
	if err := db.Create(pending).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	emailQueue := &fakeOrderTimeoutQueue{}
	svc := NewOrderService(OrderServiceOptions{
		OrderStore:       ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
		ProductStore:     productgormstore.NewProductStore(db),
		ProductSKUStore:  productgormstore.NewSKUStore(db),
		CouponStore:      coupongormstore.New(db),
		CouponUsageStore: coupongormstore.NewUsageStore(db),
		Queue:            emailQueue,
	})
	updated, err := svc.UpdateOrderStatus(order.ID, constants.OrderStatusCanceled)
	if err != nil {
		t.Fatalf("update order status failed: %v", err)
	}
	if updated == nil || updated.Status != constants.OrderStatusCanceled {
		t.Fatalf("expected canceled order, got: %+v", updated)
	}

	var reloaded paymentdomain.Payment
	if err := db.First(&reloaded, pending.ID).Error; err != nil {
		t.Fatalf("reload payment failed: %v", err)
	}
	if reloaded.Status != constants.PaymentStatusExpired || reloaded.ExpiredAt == nil {
		t.Fatalf("payment should expire, got status=%s expired_at=%v", reloaded.Status, reloaded.ExpiredAt)
	}
	if len(emailQueue.statusEmails) != 0 {
		t.Fatalf("canceling an order must not enqueue status email, got %v", emailQueue.statusEmails)
	}
}

func TestCancelExpiredOrderExpiresPaymentsForParentAndChildren(t *testing.T) {
	db := setupCancelPaymentTestDB(t, "expire_payments_parent_children")

	now := time.Now()
	expiresAt := now.Add(-time.Hour)
	parent := newPendingOrderForCancel("PARENT-EXPIRE-001", 0, nil, now)
	parent.ExpiresAt = &expiresAt
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent failed: %v", err)
	}

	childA := newPendingOrderForCancel("PARENT-EXPIRE-001-A", 0, &parent.ID, now)
	childA.ExpiresAt = &expiresAt
	if err := db.Create(childA).Error; err != nil {
		t.Fatalf("create childA failed: %v", err)
	}
	childB := newPendingOrderForCancel("PARENT-EXPIRE-001-B", 0, &parent.ID, now)
	childB.ExpiresAt = &expiresAt
	if err := db.Create(childB).Error; err != nil {
		t.Fatalf("create childB failed: %v", err)
	}

	parentPayment := newPaymentForOrder(parent.ID, constants.PaymentStatusPending, now)
	childAPayment := newPaymentForOrder(childA.ID, constants.PaymentStatusInitiated, now)
	childBPayment := newPaymentForOrder(childB.ID, constants.PaymentStatusPending, now)
	if err := db.Create([]*paymentdomain.Payment{parentPayment, childAPayment, childBPayment}).Error; err != nil {
		t.Fatalf("create payments failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		OrderStore:       ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
		ProductStore:     productgormstore.NewProductStore(db),
		ProductSKUStore:  productgormstore.NewSKUStore(db),
		CouponStore:      coupongormstore.New(db),
		CouponUsageStore: coupongormstore.NewUsageStore(db),
	})
	updated, err := svc.CancelExpiredOrder(parent.ID)
	if err != nil {
		t.Fatalf("cancel expired parent failed: %v", err)
	}
	if updated == nil || updated.Status != constants.OrderStatusCanceled {
		t.Fatalf("expected canceled parent, got: %+v", updated)
	}

	for _, pid := range []uint{parentPayment.ID, childAPayment.ID, childBPayment.ID} {
		var p paymentdomain.Payment
		if err := db.First(&p, pid).Error; err != nil {
			t.Fatalf("reload payment %d failed: %v", pid, err)
		}
		if p.Status != constants.PaymentStatusExpired || p.ExpiredAt == nil {
			t.Fatalf("payment %d should expire, got status=%s expired_at=%v", pid, p.Status, p.ExpiredAt)
		}
	}
}

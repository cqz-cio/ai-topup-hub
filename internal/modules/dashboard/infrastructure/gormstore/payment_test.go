package gormstore

import (
	"testing"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestPaymentStatsExcludeWalletProvider(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	now := time.Now().UTC().Truncate(time.Second)

	channel := &paymentdomain.PaymentChannel{
		Name:            "支付宝",
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeAlipay,
		InteractionMode: constants.PaymentInteractionRedirect,
		FeeRate:         money.FromDecimal(decimal.Zero),
		IsActive:        true,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("create channel failed: %v", err)
	}

	onlineSuccess := &paymentdomain.Payment{
		OrderID:         1,
		ChannelID:       channel.ID,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeAlipay,
		InteractionMode: constants.PaymentInteractionRedirect,
		Amount:          money.FromDecimal(decimal.NewFromInt(120)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusSuccess,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(onlineSuccess).Error; err != nil {
		t.Fatalf("create online success payment failed: %v", err)
	}

	onlineFailed := &paymentdomain.Payment{
		OrderID:         2,
		ChannelID:       channel.ID,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeAlipay,
		InteractionMode: constants.PaymentInteractionRedirect,
		Amount:          money.FromDecimal(decimal.NewFromInt(88)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusFailed,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(onlineFailed).Error; err != nil {
		t.Fatalf("create online failed payment failed: %v", err)
	}

	walletSuccess := &paymentdomain.Payment{
		OrderID:         3,
		ChannelID:       0,
		ProviderType:    constants.PaymentProviderWallet,
		ChannelType:     constants.PaymentChannelTypeBalance,
		InteractionMode: constants.PaymentInteractionBalance,
		Amount:          money.FromDecimal(decimal.NewFromInt(59)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusSuccess,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(walletSuccess).Error; err != nil {
		t.Fatalf("create wallet payment failed: %v", err)
	}
	deletedAt := now.Add(time.Minute)
	deletedOnlineSuccess := &paymentdomain.Payment{
		OrderID:         4,
		ChannelID:       channel.ID,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeAlipay,
		InteractionMode: constants.PaymentInteractionRedirect,
		Amount:          money.FromDecimal(decimal.NewFromInt(999)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusSuccess,
		CreatedAt:       now,
		UpdatedAt:       now,
		DeletedAt:       &deletedAt,
	}
	if err := db.Create(deletedOnlineSuccess).Error; err != nil {
		t.Fatalf("create soft-deleted online payment failed: %v", err)
	}

	startAt := now.Add(-time.Hour)
	endAt := now.Add(time.Hour)

	overview, err := repo.GetOverview(startAt, endAt)
	if err != nil {
		t.Fatalf("get overview failed: %v", err)
	}
	if overview.PaymentsTotal != 2 {
		t.Fatalf("payments total want 2 got %d", overview.PaymentsTotal)
	}
	if overview.PaymentsSuccess != 1 {
		t.Fatalf("payments success want 1 got %d", overview.PaymentsSuccess)
	}
	if overview.PaymentsFailed != 1 {
		t.Fatalf("payments failed want 1 got %d", overview.PaymentsFailed)
	}

	trends, err := repo.GetPaymentTrends(startAt, endAt)
	if err != nil {
		t.Fatalf("get payment trends failed: %v", err)
	}
	if len(trends) == 0 {
		t.Fatalf("payment trends should not be empty")
	}
	point := trends[0]
	if point.PaymentsSuccess != 1 {
		t.Fatalf("trend payments success want 1 got %d", point.PaymentsSuccess)
	}
	if point.PaymentsFailed != 1 {
		t.Fatalf("trend payments failed want 1 got %d", point.PaymentsFailed)
	}
	if point.GMVPaid != 120 {
		t.Fatalf("trend paid amount want 120 got %.2f", point.GMVPaid)
	}

	topChannels, err := repo.GetTopChannels(startAt, endAt, 5)
	if err != nil {
		t.Fatalf("get top channels failed: %v", err)
	}
	if len(topChannels) != 1 {
		t.Fatalf("top channels len want 1 got %d", len(topChannels))
	}
	if topChannels[0].ProviderType != constants.PaymentProviderOfficial {
		t.Fatalf("top channel provider want %s got %s", constants.PaymentProviderOfficial, topChannels[0].ProviderType)
	}
}

func TestGetPaymentOrderAlertCountsExcludesChildOrdersAndWalletPayments(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	now := time.Now().UTC().Truncate(time.Second)

	parentOrder := &orderdomain.Order{
		OrderNo:        "DJ-PENDING-001",
		Status:         constants.OrderStatusPendingPayment,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(parentOrder).Error; err != nil {
		t.Fatalf("create parent pending order failed: %v", err)
	}

	childOrder := &orderdomain.Order{
		OrderNo:        "DJ-PENDING-001-01",
		ParentID:       &parentOrder.ID,
		Status:         constants.OrderStatusPendingPayment,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(childOrder).Error; err != nil {
		t.Fatalf("create child pending order failed: %v", err)
	}

	onlineFailed := &paymentdomain.Payment{
		OrderID:         parentOrder.ID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeAlipay,
		InteractionMode: constants.PaymentInteractionRedirect,
		Amount:          money.FromDecimal(decimal.NewFromInt(100)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusFailed,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(onlineFailed).Error; err != nil {
		t.Fatalf("create online failed payment failed: %v", err)
	}

	walletFailed := &paymentdomain.Payment{
		OrderID:         parentOrder.ID,
		ProviderType:    constants.PaymentProviderWallet,
		ChannelType:     constants.PaymentChannelTypeBalance,
		InteractionMode: constants.PaymentInteractionBalance,
		Amount:          money.FromDecimal(decimal.NewFromInt(100)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusFailed,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(walletFailed).Error; err != nil {
		t.Fatalf("create wallet failed payment failed: %v", err)
	}

	counts, err := repo.GetPaymentOrderAlertCounts(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("get payment order alert counts failed: %v", err)
	}
	if counts.PendingPaymentOrders != 1 {
		t.Fatalf("pending payment orders want 1 got %d", counts.PendingPaymentOrders)
	}
	if counts.PaymentsFailed != 1 {
		t.Fatalf("payments failed want 1 got %d", counts.PaymentsFailed)
	}
}

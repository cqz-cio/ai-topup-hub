package gormstore

import (
	"fmt"
	"testing"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestTrendQueriesBucketByRequestedTimezone(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location failed: %v", err)
	}
	baseUTC := time.Date(2026, 3, 1, 15, 30, 0, 0, time.UTC)
	nextUTC := time.Date(2026, 3, 1, 16, 30, 0, 0, time.UTC)

	for idx, createdAt := range []time.Time{baseUTC, nextUTC} {
		order := &orderdomain.Order{
			OrderNo:        fmt.Sprintf("DJ-TZ-%d", idx),
			UserID:         1,
			Status:         constants.OrderStatusPaid,
			Currency:       "CNY",
			OriginalAmount: money.FromDecimal(decimal.NewFromInt(50)),
			DiscountAmount: money.FromDecimal(decimal.Zero),
			TotalAmount:    money.FromDecimal(decimal.NewFromInt(50)),
			CreatedAt:      createdAt,
		}
		if err := db.Create(order).Error; err != nil {
			t.Fatalf("create order failed: %v", err)
		}
	}

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
	for idx, item := range []struct {
		createdAt time.Time
		status    string
		amount    int64
	}{
		{createdAt: baseUTC, status: constants.PaymentStatusSuccess, amount: 30},
		{createdAt: nextUTC, status: constants.PaymentStatusFailed, amount: 40},
	} {
		payment := &paymentdomain.Payment{
			OrderID:         uint(idx + 1),
			ChannelID:       channel.ID,
			ProviderType:    constants.PaymentProviderOfficial,
			ChannelType:     constants.PaymentChannelTypeAlipay,
			InteractionMode: constants.PaymentInteractionRedirect,
			Amount:          money.FromDecimal(decimal.NewFromInt(item.amount)),
			FeeRate:         money.FromDecimal(decimal.Zero),
			FeeAmount:       money.FromDecimal(decimal.Zero),
			Currency:        "CNY",
			Status:          item.status,
			CreatedAt:       item.createdAt,
			UpdatedAt:       item.createdAt,
		}
		if err := db.Create(payment).Error; err != nil {
			t.Fatalf("create payment failed: %v", err)
		}
	}

	startAt := time.Date(2026, 3, 1, 0, 0, 0, 0, location)
	endAt := time.Date(2026, 3, 3, 0, 0, 0, 0, location)

	orderRows, err := repo.GetOrderTrends(startAt, endAt)
	if err != nil {
		t.Fatalf("get order trends failed: %v", err)
	}
	if len(orderRows) != 2 {
		t.Fatalf("order trend rows want 2 got %d", len(orderRows))
	}
	if orderRows[0].Day != "2026-03-01" || orderRows[0].OrdersTotal != 1 || orderRows[0].OrdersPaid != 1 {
		t.Fatalf("unexpected first order trend row: %+v", orderRows[0])
	}
	if orderRows[1].Day != "2026-03-02" || orderRows[1].OrdersTotal != 1 || orderRows[1].OrdersPaid != 1 {
		t.Fatalf("unexpected second order trend row: %+v", orderRows[1])
	}

	paymentRows, err := repo.GetPaymentTrends(startAt, endAt)
	if err != nil {
		t.Fatalf("get payment trends failed: %v", err)
	}
	if len(paymentRows) != 2 {
		t.Fatalf("payment trend rows want 2 got %d", len(paymentRows))
	}
	if paymentRows[0].Day != "2026-03-01" || paymentRows[0].PaymentsSuccess != 1 || paymentRows[0].PaymentsFailed != 0 || paymentRows[0].GMVPaid != 30 {
		t.Fatalf("unexpected first payment trend row: %+v", paymentRows[0])
	}
	if paymentRows[1].Day != "2026-03-02" || paymentRows[1].PaymentsSuccess != 0 || paymentRows[1].PaymentsFailed != 1 || paymentRows[1].GMVPaid != 0 {
		t.Fatalf("unexpected second payment trend row: %+v", paymentRows[1])
	}
}

package salereader

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/notification/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/upstream"
	"github.com/shopspring/decimal"
)

type orderStub struct{ order *orderdomain.Order }

func (s orderStub) GetByID(uint) (*orderdomain.Order, error) { return s.order, nil }

type procurementStub map[uint]*procurementdomain.Order

func (s procurementStub) GetByLocalOrderID(id uint) (*procurementdomain.Order, error) {
	return s[id], nil
}

type balanceAdapter struct {
	upstream.Adapter // Any accidental purchase call panics instead of reaching a supplier.
	result           *upstream.PingResult
	err              error
	calls            int
	bounded          bool
	stall            bool
}

func (a *balanceAdapter) Ping(ctx context.Context) (*upstream.PingResult, error) {
	a.calls++
	deadline, ok := ctx.Deadline()
	a.bounded = ok && time.Until(deadline) <= 8*time.Second
	if a.stall {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return a.result, a.err
}

func TestSlowSupplierCannotDelayMailBeyondBalanceBudget(t *testing.T) {
	order := &orderdomain.Order{ID: 1, Children: []orderdomain.Order{upstreamOrder(2), upstreamOrder(3)}}
	adapter := &balanceAdapter{stall: true}
	r := New(orderStub{order}, procurementStub{2: purchase(8, "fulfilled", "33", "CNY"), 3: purchase(9, "fulfilled", "10", "CNY")}, connectionStub{8: adapter, 9: adapter})
	start := time.Now()
	data, err := r.Build(context.Background(), 1, constants.LocaleZhCN, true)
	if err != nil || time.Since(start) > 4*time.Second || !strings.Contains(data["upstream_summary"].(string), "余额暂时查询失败") {
		t.Fatalf("slow supplier blocked notification: %v %v", err, data)
	}
}

type connectionStub map[uint]*balanceAdapter

func (s connectionStub) GetByID(id uint) (*siteconnectiondomain.Connection, error) {
	return &siteconnectiondomain.Connection{ID: id, Name: "测试上游"}, nil
}
func (s connectionStub) GetAdapter(c *siteconnectiondomain.Connection) (upstream.Adapter, error) {
	return s[c.ID], nil
}

func upstreamOrder(id uint) orderdomain.Order {
	return orderdomain.Order{ID: id, OrderNo: "SALE", Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeUpstream}}}
}

func purchase(id uint, status, amount, currency string) *procurementdomain.Order {
	return &procurementdomain.Order{ConnectionID: id, LocalOrderNo: "SALE", UpstreamOrderNo: "SUPPLIER-1", Status: status,
		UpstreamAmount: money.Amount{Decimal: decimal.RequireFromString(amount)}, UpstreamCurrency: currency,
		UpstreamPayload: "CARD-MUST-NOT-APPEAR"}
}

func TestSaleSummaryWaitsForAllPurchasesBeforeQueryingBalance(t *testing.T) {
	order := &orderdomain.Order{ID: 1, Children: []orderdomain.Order{upstreamOrder(2), upstreamOrder(3)}}
	adapter := &balanceAdapter{result: &upstream.PingResult{Balance: "67", Currency: "CNY"}}
	orders := procurementStub{2: purchase(9, "delivered", "33", "CNY"), 3: purchase(9, "pending", "0", "CNY")}
	r := New(orderStub{order}, orders, connectionStub{9: adapter})
	if _, err := r.Build(context.Background(), 1, constants.LocaleZhCN, false); !errors.Is(err, contract.ErrProcurementPending) {
		t.Fatalf("want pending, got %v", err)
	}
	if adapter.calls != 0 {
		t.Fatal("queried balance before all purchases were settled")
	}
	orders[3] = purchase(9, "accepted", "20", "CNY")
	data, err := r.Build(context.Background(), 1, constants.LocaleZhCN, false)
	if err != nil {
		t.Fatal(err)
	}
	text := data["upstream_summary"].(string)
	for _, want := range []string{"53.00 CNY", "67.00 CNY", "采购已确认", "+08:00"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	if adapter.calls != 1 || !adapter.bounded {
		t.Fatal("expected one bounded read per supplier account")
	}
	if strings.Contains(text, "CARD-MUST-NOT-APPEAR") {
		t.Fatal("card leaked into administrator sale summary")
	}
}

func TestSaleSummaryMissingPurchaseAndLongWait(t *testing.T) {
	order := upstreamOrder(1)
	r := New(orderStub{&order}, procurementStub{}, connectionStub{})
	if _, err := r.Build(context.Background(), 1, constants.LocaleZhCN, false); !errors.Is(err, contract.ErrProcurementPending) {
		t.Fatal(err)
	}
	data, err := r.Build(context.Background(), 1, constants.LocaleZhCN, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data["upstream_summary"].(string), "采购尚未完成") {
		t.Fatal(data)
	}
}

func TestSaleSummaryBalanceFailureDoesNotInventZeroOrFailNotification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result *upstream.PingResult
		err    error
	}{
		{"network", nil, errors.New("temporary connection error")},
		{"nil", nil, nil},
		{"invalid", &upstream.PingResult{Balance: "bad", Currency: "CNY"}, nil},
		{"negative", &upstream.PingResult{Balance: "-1", Currency: "CNY"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			order := upstreamOrder(1)
			r := New(orderStub{&order}, procurementStub{1: purchase(9, "delivered", "33", "CNY")}, connectionStub{9: &balanceAdapter{result: tc.result, err: tc.err}})
			data, err := r.Build(context.Background(), 1, constants.LocaleZhCN, false)
			if err != nil {
				t.Fatal(err)
			}
			text := data["upstream_balances"].(string)
			if !strings.Contains(text, "查询失败") || strings.Contains(text, "0.00") {
				t.Fatal(text)
			}
		})
	}
}

func TestSaleSummaryKeepsActualZeroAndUncertainPurchaseSeparate(t *testing.T) {
	order := upstreamOrder(1)
	r := New(orderStub{&order}, procurementStub{1: purchase(9, "manual_review", "0", "CNY")}, connectionStub{9: &balanceAdapter{result: &upstream.PingResult{Balance: "0", Currency: "CNY"}}})
	data, err := r.Build(context.Background(), 1, constants.LocaleZhCN, false)
	if err != nil {
		t.Fatal(err)
	}
	text := data["upstream_summary"].(string)
	if !strings.Contains(text, "结果不明") || !strings.Contains(text, "剩余余额：0.00 CNY") || data["upstream_purchase_amounts"] != "" {
		t.Fatal(text)
	}
}

func TestSaleSummaryDoesNotMixCurrenciesOrReadBalanceForLocalGoods(t *testing.T) {
	order := &orderdomain.Order{ID: 1, Children: []orderdomain.Order{upstreamOrder(2), upstreamOrder(3)}}
	first := &balanceAdapter{result: &upstream.PingResult{Balance: "50", Currency: "CNY"}}
	second := &balanceAdapter{result: &upstream.PingResult{Balance: "10", Currency: "USD"}}
	r := New(orderStub{order}, procurementStub{2: purchase(8, "delivered", "33", "CNY"), 3: purchase(9, "delivered", "5", "USD")}, connectionStub{8: first, 9: second})
	data, err := r.Build(context.Background(), 1, constants.LocaleEnUS, false)
	if err != nil {
		t.Fatal(err)
	}
	if data["upstream_purchase_amounts"] != "33.00 CNY; 5.00 USD" || first.calls != 1 || second.calls != 1 {
		t.Fatal(data)
	}
	r.orders = orderStub{&orderdomain.Order{ID: 1, Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeAuto}}}}
	data, err = r.Build(context.Background(), 1, constants.LocaleZhCN, false)
	if err != nil || len(data) != 0 || first.calls != 1 {
		t.Fatalf("unexpected upstream read for local goods: %v %v", data, err)
	}
}

func TestFulfilledProcurementIsFinalAndIncludesActualCost(t *testing.T) {
	for locale, delivered := range map[string]string{constants.LocaleZhCN: "已发货", constants.LocaleZhTW: "已發貨", constants.LocaleEnUS: "Delivered"} {
		t.Run(locale, func(t *testing.T) {
			order := upstreamOrder(1)
			adapter := &balanceAdapter{result: &upstream.PingResult{Balance: "67", Currency: "CNY"}}
			r := New(orderStub{&order}, procurementStub{1: purchase(9, "fulfilled", "33", "CNY")}, connectionStub{9: adapter})
			data, err := r.Build(context.Background(), 1, locale, false)
			if err != nil {
				t.Fatal(err)
			}
			text := data["upstream_summary"].(string)
			if data["upstream_purchase_amounts"] != "33.00 CNY" || !strings.Contains(text, delivered) || strings.Contains(text, "CARD-MUST-NOT-APPEAR") || adapter.calls != 1 {
				t.Fatalf("incorrect completed purchase summary: %v", data)
			}
		})
	}
}

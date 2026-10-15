package application

import (
	"context"
	"errors"
	"testing"

	"github.com/dujiao-next/internal/constants"
	pc "github.com/dujiao-next/internal/modules/procurement/contract"
	pd "github.com/dujiao-next/internal/modules/procurement/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type readinessOrders struct {
	pc.OrderRepository
	order *pd.LocalOrder
}

func (r readinessOrders) GetByID(uint) (*pd.LocalOrder, error) { return r.order, nil }

type readinessMappings map[uint]uint

func (m readinessMappings) FindConnectionID(id uint) (uint, bool, error) {
	v, ok := m[id]
	return v, ok, nil
}

type readinessBalance struct {
	pc.UpstreamConnection
	amount, currency string
	err              error
	calls            int
}

func (b *readinessBalance) ReadBalance(context.Context) (string, string, error) {
	b.calls++
	return b.amount, b.currency, b.err
}

type readinessConnections map[uint]*readinessBalance

func (m readinessConnections) Open(id uint) (pc.UpstreamConnection, error) {
	b := m[id]
	if b == nil {
		return nil, errors.New("unavailable")
	}
	return b, nil
}

type readinessNotifier struct{ calls int }

func (n *readinessNotifier) NotifyBalanceShortage(context.Context, uint, string, string, string) string {
	n.calls++
	return "pending"
}
func (*readinessNotifier) BalanceAlertStatus(context.Context, uint) string { return "sent" }

func readinessItem(product uint, cost string, quantity int) pd.LocalOrderItem {
	return pd.LocalOrderItem{ProductID: product, CostPrice: money.FromDecimal(decimal.RequireFromString(cost)), Quantity: quantity, FulfillmentType: constants.FulfillmentTypeUpstream}
}

func TestPaymentBalanceAggregatesLeavesPerSupplier(t *testing.T) {
	for _, tc := range []struct{ name, balance, code string }{
		{"insufficient", "119.99", pc.BalanceInsufficient}, {"exact", "120.00", ""}, {"surplus", "120.01", ""},
		{"zero", "0", pc.BalanceInsufficient}, {"malformed", "oops", pc.BalanceUnavailable}, {"negative", "-1", pc.BalanceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item1, item2 := readinessItem(1, "40", 2), readinessItem(2, "40", 1)
			order := &pd.LocalOrder{ID: 1, Currency: "CNY", Items: []pd.LocalOrderItem{item1, item2}, Children: []pd.LocalOrder{
				{Currency: "CNY", Items: []pd.LocalOrderItem{item1}}, {Currency: "CNY", Items: []pd.LocalOrderItem{item2}},
			}}
			balance := &readinessBalance{amount: tc.balance, currency: "CNY"}
			notifier := &readinessNotifier{}
			svc := NewService(Options{Orders: readinessOrders{order: order}, ProductMappings: readinessMappings{1: 7, 2: 7}, Connections: readinessConnections{7: balance}})
			svc.SetBalanceNotifier(notifier)
			err := svc.CheckPaymentBalance(context.Background(), 1)
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var block *pc.PaymentBlock
				if !errors.As(err, &block) || block.Code != tc.code {
					t.Fatalf("unexpected failure: %v", err)
				}
				if tc.code == pc.BalanceInsufficient && block.NotificationStatus != "pending" {
					t.Fatalf("queue is not sent: %+v", block)
				}
			}
			if balance.calls != 1 {
				t.Fatalf("balance queries = %d", balance.calls)
			}
			want := 0
			if tc.code == pc.BalanceInsufficient {
				want = 1
			}
			if notifier.calls != want {
				t.Fatalf("mail requests = %d", notifier.calls)
			}
			_ = svc.ReadPaymentBalanceNotice(context.Background(), 1, 7)
			if svc.ReadPaymentBalanceNotice(context.Background(), 1, 999) != "not_sent" {
				t.Fatal("exposed another supplier notice")
			}
			if balance.calls != 1 || notifier.calls != want {
				t.Fatal("notice polling caused supplier or email activity")
			}
		})
	}
}

func TestPaymentBalanceFailsClosedAndChecksEverySupplier(t *testing.T) {
	for _, tc := range []struct {
		name     string
		item     pd.LocalOrderItem
		mappings readinessMappings
		balances readinessConnections
		code     string
	}{
		{"missing mapping", readinessItem(1, "10", 1), readinessMappings{}, nil, pc.BalanceUnavailable},
		{"missing cost", readinessItem(1, "0", 1), readinessMappings{1: 1}, nil, pc.BalanceUnavailable},
		{"currency mismatch", readinessItem(1, "10", 1), readinessMappings{1: 1}, readinessConnections{1: {amount: "100", currency: "USD"}}, pc.BalanceUnavailable},
		{"query failure", readinessItem(1, "10", 1), readinessMappings{1: 1}, readinessConnections{1: {err: context.DeadlineExceeded}}, pc.BalanceUnavailable},
		{"second supplier low", readinessItem(1, "10", 1), readinessMappings{1: 1, 2: 2}, readinessConnections{1: {amount: "100", currency: "CNY"}, 2: {amount: "19", currency: "CNY"}}, pc.BalanceInsufficient},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := []pd.LocalOrderItem{tc.item}
			if tc.name == "second supplier low" {
				items = append(items, readinessItem(2, "20", 1))
			}
			svc := NewService(Options{Orders: readinessOrders{order: &pd.LocalOrder{Currency: "CNY", Items: items}}, ProductMappings: tc.mappings, Connections: tc.balances})
			var block *pc.PaymentBlock
			if err := svc.CheckPaymentBalance(context.Background(), 1); !errors.As(err, &block) || block.Code != tc.code {
				t.Fatalf("unexpected failure: %v", err)
			}
		})
	}
	local := NewService(Options{Orders: readinessOrders{order: &pd.LocalOrder{Items: []pd.LocalOrderItem{{FulfillmentType: constants.FulfillmentTypeAuto}}}}})
	if err := local.CheckPaymentBalance(context.Background(), 1); err != nil {
		t.Fatalf("local stock blocked: %v", err)
	}
}

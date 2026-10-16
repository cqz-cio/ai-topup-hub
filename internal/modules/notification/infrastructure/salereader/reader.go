// Package salereader builds an administrator-only procurement projection.
// It has no purchase, order mutation, or customer email capabilities.
package salereader

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/notification/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/upstream"
	"github.com/shopspring/decimal"
)

type Orders interface {
	GetByID(uint) (*orderdomain.Order, error)
}

type Procurements interface {
	GetByLocalOrderID(uint) (*procurementdomain.Order, error)
}

type Connections interface {
	GetByID(uint) (*siteconnectiondomain.Connection, error)
	GetAdapter(*siteconnectiondomain.Connection) (upstream.Adapter, error)
}

type Reader struct {
	orders       Orders
	procurements Procurements
	connections  Connections
	now          func() time.Time
}

func New(orders Orders, procurements Procurements, connections Connections) *Reader {
	return &Reader{orders: orders, procurements: procurements, connections: connections, now: time.Now}
}

var _ contract.SaleSummaryReader = (*Reader)(nil)

func (r *Reader) Build(ctx context.Context, orderID uint, locale string, allowPending bool) (map[string]interface{}, error) {
	order, err := r.orders.GetByID(orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, errors.New("sale notification order not found")
	}
	leaves := []*orderdomain.Order{order}
	if len(order.Children) > 0 {
		leaves = nil
		for i := range order.Children {
			leaves = append(leaves, &order.Children[i])
		}
	}
	var purchases []*procurementdomain.Order
	for _, leaf := range leaves {
		hasUpstream := false
		for _, item := range leaf.Items {
			if strings.TrimSpace(item.FulfillmentType) == constants.FulfillmentTypeUpstream {
				hasUpstream = true
				break
			}
		}
		if !hasUpstream {
			continue
		}
		purchase, err := r.procurements.GetByLocalOrderID(leaf.ID)
		if err != nil {
			return nil, err
		}
		if purchase == nil {
			purchase = &procurementdomain.Order{LocalOrderNo: leaf.OrderNo, Status: "pending"}
		}
		if pending(purchase.Status) && !allowPending {
			return nil, contract.ErrProcurementPending
		}
		purchases = append(purchases, purchase)
	}
	if len(purchases) == 0 {
		return nil, nil
	}
	labels := labelsFor(locale)
	lines := []string{labels.heading}
	confirmed := map[string]decimal.Decimal{}
	accounts := map[uint]bool{}
	for _, purchase := range purchases {
		status := labels.status(purchase.Status)
		line := fmt.Sprintf("%s：%s — %s", labels.order, purchase.LocalOrderNo, status)
		if purchase.UpstreamOrderNo != "" {
			line += fmt.Sprintf(" / %s：%s", labels.reference, purchase.UpstreamOrderNo)
		}
		switch purchase.Status {
		case "accepted", "fulfilled", "delivered", "refunded", "partially_refunded":
			currency := strings.ToUpper(strings.TrimSpace(purchase.UpstreamCurrency))
			if currency != "" && purchase.UpstreamAmount.Decimal.GreaterThan(decimal.Zero) {
				line += fmt.Sprintf(" / %s：%s %s", labels.cost, purchase.UpstreamAmount.Decimal.StringFixed(2), currency)
				confirmed[currency] = confirmed[currency].Add(purchase.UpstreamAmount.Decimal)
			} else {
				line += " / " + labels.costUnknown
			}
		}
		lines = append(lines, line)
		if purchase.ConnectionID > 0 {
			accounts[purchase.ConnectionID] = true
		}
	}
	currencies := make([]string, 0, len(confirmed))
	for currency := range confirmed {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	totals := make([]string, 0, len(currencies))
	for _, currency := range currencies {
		totals = append(totals, confirmed[currency].StringFixed(2)+" "+currency)
	}
	if len(totals) > 0 {
		lines = append(lines, labels.total+"："+strings.Join(totals, "; "))
	}
	ids := make([]uint, 0, len(accounts))
	for id := range accounts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	// Bound all balance lookups together; these reads are independent of delivery.
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var balances []string
	for _, id := range ids {
		name := fmt.Sprintf("%s #%d", labels.account, id)
		balance, currency, ok := "", "", false
		connection, connErr := r.connections.GetByID(id)
		if connErr == nil && connection != nil {
			if strings.TrimSpace(connection.Name) != "" {
				name = connection.Name
			}
			adapter, adapterErr := r.connections.GetAdapter(connection)
			if adapterErr == nil && adapter != nil {
				accountCtx, accountCancel := context.WithTimeout(queryCtx, 3*time.Second)
				result, pingErr := adapter.Ping(accountCtx)
				accountCancel()
				if pingErr == nil && result != nil {
					amount, parseErr := decimal.NewFromString(result.Balance)
					currency = strings.ToUpper(strings.TrimSpace(result.Currency))
					if parseErr == nil && !amount.IsNegative() && currency != "" {
						balance, ok = amount.StringFixed(2), true
					}
				}
			}
		}
		checkedAt := r.now().In(time.FixedZone("UTC+08:00", 8*60*60)).Format("2006-01-02 15:04:05 +08:00")
		value := labels.unavailable
		if ok {
			value = balance + " " + currency
		}
		balances = append(balances, fmt.Sprintf("%s · %s：%s\n%s：%s", name, labels.balance, value, labels.checkedAt, checkedAt))
	}
	if len(ids) == 0 {
		balances = append(balances, labels.unavailable)
	}
	lines = append(lines, balances...)
	lines = append(lines, labels.snapshot)
	return map[string]interface{}{
		"upstream_summary":          strings.Join(lines, "\n"),
		"upstream_purchase_amounts": strings.Join(totals, "; "),
		"upstream_balances":         strings.Join(balances, "\n"),
	}, nil
}

func pending(status string) bool {
	switch status {
	case "accepted", "fulfilled", "delivered", "rejected", "manual_review", "canceled", "refunded", "partially_refunded":
		return false
	default:
		return true
	}
}

type labels struct {
	heading, order, reference, cost, costUnknown, total, account, balance, checkedAt, unavailable, snapshot string
	statuses                                                                                                map[string]string
}

func (l labels) status(value string) string {
	// Procurement records use fulfilled; delivered is the customer order state.
	if value == "fulfilled" {
		value = "delivered"
	}
	if text, ok := l.statuses[value]; ok {
		return text
	}
	return l.statuses["pending"]
}

func labelsFor(locale string) labels {
	if locale == constants.LocaleEnUS {
		return labels{"Supplier purchase and balance", "Order", "Supplier order", "Purchase amount", "Purchase amount unavailable", "Confirmed purchase total (before refunds)", "Supplier account", "Remaining balance", "Checked at", "Balance query unavailable", "Balances are snapshots at the stated times; other purchases or top-ups may change them.", map[string]string{"accepted": "Purchase confirmed; delivery pending", "delivered": "Delivered", "rejected": "Purchase failed; action required", "manual_review": "Purchase outcome unknown; reconcile with supplier", "canceled": "Canceled", "refunded": "Refunded", "partially_refunded": "Partially refunded", "pending": "Procurement still pending; check the order"}}
	}
	if locale == constants.LocaleZhTW {
		return labels{"上游採購與帳戶餘額", "訂單", "上游訂單", "採購金額", "採購金額待核對", "已確認採購合計（退款前）", "上游帳戶", "剩餘餘額", "查詢時間（北京時間）", "餘額暫時查詢失敗，請到上游核對", "餘額為上述時間的查詢結果，其他訂單或儲值可能繼續改變餘額。", map[string]string{"accepted": "採購已確認，等待發貨", "delivered": "已發貨", "rejected": "採購失敗，請處理", "manual_review": "採購結果不明，請核對原交易", "canceled": "已取消", "refunded": "已退款", "partially_refunded": "部分退款", "pending": "採購尚未完成，請關注訂單"}}
	}
	return labels{"上游采购与账户余额", "订单", "上游订单", "采购金额", "采购金额待核对", "已确认采购合计（退款前）", "上游账户", "剩余余额", "查询时间（北京时间）", "余额暂时查询失败，请到上游核对", "余额为上述时间的查询结果，其他订单或充值可能继续改变余额。", map[string]string{"accepted": "采购已确认，等待发货", "delivered": "已发货", "rejected": "采购失败，请处理", "manual_review": "采购结果不明，请核对原交易", "canceled": "已取消", "refunded": "已退款", "partially_refunded": "部分退款", "pending": "采购尚未完成，请关注订单"}}
}

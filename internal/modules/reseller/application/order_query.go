package application

import (
	"fmt"
	"strings"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
)

// OrderQueryService 分销销售订单只读查询用例。
type OrderQueryService struct {
	store resellercontract.OrderQueryStore
}

func NewOrderQueryService(store resellercontract.OrderQueryStore) *OrderQueryService {
	return &OrderQueryService{store: store}
}

type pricingItemSnapshot struct {
	BaseUnitAmount      string
	ResellerUnitAmount  string
	BaseTotalAmount     string
	ResellerTotalAmount string
	ProfitAmount        string
}

func (s *OrderQueryService) requireActiveProfileByUser(userID uint) (*resellerdomain.Profile, error) {
	if s == nil || s.store == nil || userID == 0 {
		return nil, resellercontract.ErrNotOpened
	}
	profile, err := s.store.GetProfileByUserID(userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, resellercontract.ErrNotOpened
	}
	// 订单只读视角仅要求资料激活，不校验结算冻结（与提现/账务不同）。
	if profile.Status != resellerdomain.ProfileStatusActive {
		return nil, resellercontract.ErrProfileInactive
	}
	return profile, nil
}

func (s *OrderQueryService) ListUserOrders(userID uint, input resellercontract.OrderListInput) ([]resellercontract.OrderListItem, int64, error) {
	profile, err := s.requireActiveProfileByUser(userID)
	if err != nil {
		return nil, 0, err
	}
	rows, total, err := s.store.ListOrderSnapshotsByReseller(orderSnapshotFilter(profile.ID, input))
	if err != nil {
		return nil, 0, err
	}
	out := make([]resellercontract.OrderListItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, buildOrderListItem(row))
	}
	return out, total, nil
}

func (s *OrderQueryService) ListAdminOrders(resellerID uint, input resellercontract.OrderListInput) ([]resellercontract.OrderListItem, int64, error) {
	if s == nil || s.store == nil || resellerID == 0 {
		return nil, 0, productcontract.ErrNotFound
	}
	profile, err := s.store.GetProfileByID(resellerID)
	if err != nil {
		return nil, 0, err
	}
	if profile == nil {
		return nil, 0, productcontract.ErrNotFound
	}
	rows, total, err := s.store.ListOrderSnapshotsByReseller(orderSnapshotFilter(resellerID, input))
	if err != nil {
		return nil, 0, err
	}
	out := make([]resellercontract.OrderListItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, buildOrderListItem(row))
	}
	return out, total, nil
}

func (s *OrderQueryService) GetUserOrderDetail(userID uint, orderNo string) (*resellercontract.OrderDetail, error) {
	profile, err := s.requireActiveProfileByUser(userID)
	if err != nil {
		return nil, err
	}
	row, err := s.store.GetOrderSnapshotByResellerOrderNo(profile.ID, orderNo)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, resellercontract.ErrOrderNotFound
	}
	detail := &resellercontract.OrderDetail{OrderListItem: buildOrderListItem(*row)}
	detail.Items = buildOrderItemDetails(*row)
	return detail, nil
}

func (s *OrderQueryService) StatsUserOrders(userID uint, input resellercontract.OrderListInput) (resellercontract.OrderStats, error) {
	profile, err := s.requireActiveProfileByUser(userID)
	if err != nil {
		return resellercontract.OrderStats{}, err
	}
	row, err := s.store.StatsOrderSnapshotsByReseller(orderSnapshotFilter(profile.ID, input))
	if err != nil {
		return resellercontract.OrderStats{}, err
	}
	return resellercontract.OrderStats(row), nil
}

func orderSnapshotFilter(resellerID uint, input resellercontract.OrderListInput) resellercontract.OrderSnapshotListFilter {
	return resellercontract.OrderSnapshotListFilter{
		ResellerID:  resellerID,
		Page:        input.Page,
		PageSize:    input.PageSize,
		Status:      strings.TrimSpace(input.Status),
		OrderNo:     strings.TrimSpace(input.OrderNo),
		CreatedFrom: input.CreatedFrom,
		CreatedTo:   input.CreatedTo,
		PaidFrom:    input.PaidFrom,
		PaidTo:      input.PaidTo,
	}
}

func buildOrderListItem(row resellercontract.OrderSnapshotRow) resellercontract.OrderListItem {
	order := row.Order
	snapshot := row.Snapshot
	return resellercontract.OrderListItem{
		OrderNo:      order.OrderNo,
		Status:       order.Status,
		Currency:     snapshot.Currency,
		TotalAmount:  order.TotalAmount,
		BaseAmount:   snapshot.BaseAmount,
		ProfitAmount: snapshot.ProfitAmount,
		ProfitStatus: neutralProfitStatus(snapshot, order, row.LedgerEntries),
		Domain:       snapshot.Domain,
		BuyerLabel:   maskBuyerLabel(order, row.BuyerEmail),
		ItemsCount:   len(row.Items),
		CreatedAt:    order.CreatedAt,
		PaidAt:       order.PaidAt,
	}
}

func neutralProfitStatus(snapshot resellerdomain.OrderSnapshot, order orderdomain.Order, ledgerEntries []resellerdomain.LedgerEntry) string {
	if !snapshot.ProfitEligible || snapshot.ProfitAmount.Decimal.LessThanOrEqual(decimal.Zero) {
		return resellercontract.ProfitStatusUnavailable
	}
	switch order.Status {
	case constants.OrderStatusCanceled, constants.OrderStatusRefunded, constants.OrderStatusPartiallyRefunded:
		return resellercontract.ProfitStatusUnavailable
	}
	if order.PaidAt == nil || order.Status == constants.OrderStatusPendingPayment {
		return resellercontract.ProfitStatusPending
	}
	for _, entry := range ledgerEntries {
		if entry.Type != resellerdomain.LedgerTypeOrderProfit {
			continue
		}
		switch entry.Status {
		case resellerdomain.LedgerStatusAvailable, resellerdomain.LedgerStatusLocked, resellerdomain.LedgerStatusWithdrawn:
			return resellercontract.ProfitStatusCredited
		case resellerdomain.LedgerStatusPendingConfirm:
			return resellercontract.ProfitStatusPending
		case resellerdomain.LedgerStatusCanceled:
			return resellercontract.ProfitStatusUnavailable
		}
	}
	return resellercontract.ProfitStatusPending
}

func maskBuyerLabel(order orderdomain.Order, buyerEmail string) string {
	if order.UserID > 0 {
		if label := maskBuyerEmail(buyerEmail); label != "" {
			return label
		}
		return fmt.Sprintf("user#%d", order.UserID)
	}
	if label := maskBuyerEmail(order.GuestEmail); label != "" {
		return label
	}
	return "guest"
}

func maskBuyerEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	prefix := parts[0]
	if len(prefix) > 1 {
		prefix = prefix[:1]
	}
	return prefix + "***@" + parts[1]
}

func buildOrderItemDetails(row resellercontract.OrderSnapshotRow) []resellercontract.OrderItemDetail {
	pricingByItemID := pricingSnapshotByOrderItemID(row.Snapshot.PricingSnapshotJSON)
	out := make([]resellercontract.OrderItemDetail, 0, len(row.Items))
	for i := range row.Items {
		item := row.Items[i]
		itemPricing := pricingByItemID[item.ID]
		out = append(out, resellercontract.OrderItemDetail{
			Title:               item.TitleJSON,
			SKUSnapshot:         item.SKUSnapshotJSON,
			Quantity:            item.Quantity,
			UnitPrice:           item.UnitPrice,
			TotalPrice:          item.TotalPrice,
			BaseUnitAmount:      itemPricing.BaseUnitAmount,
			ResellerUnitAmount:  itemPricing.ResellerUnitAmount,
			BaseTotalAmount:     itemPricing.BaseTotalAmount,
			ResellerTotalAmount: itemPricing.ResellerTotalAmount,
			ProfitAmount:        itemPricing.ProfitAmount,
		})
	}
	return out
}

func pricingSnapshotByOrderItemID(snapshot jsonmap.JSON) map[uint]pricingItemSnapshot {
	out := map[uint]pricingItemSnapshot{}
	rawItems, ok := snapshot["items"].([]interface{})
	if !ok {
		return out
	}
	for _, raw := range rawItems {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		itemID := uintFromOrderSnapshotValue(item["order_item_id"])
		if itemID == 0 {
			continue
		}
		out[itemID] = pricingItemSnapshot{
			BaseUnitAmount:      orderSnapshotStringValue(item["base_unit_amount"]),
			ResellerUnitAmount:  orderSnapshotStringValue(item["reseller_unit_amount"]),
			BaseTotalAmount:     orderSnapshotStringValue(item["base_total_amount"]),
			ResellerTotalAmount: orderSnapshotStringValue(item["reseller_total_amount"]),
			ProfitAmount:        orderSnapshotStringValue(item["profit_amount"]),
		}
	}
	return out
}

func uintFromOrderSnapshotValue(value interface{}) uint {
	switch v := value.(type) {
	case uint:
		return v
	case int:
		if v > 0 {
			return uint(v)
		}
	case int64:
		if v > 0 {
			return uint(v)
		}
	case float64:
		if v > 0 {
			return uint(v)
		}
	case string:
		parsed, err := decimal.NewFromString(strings.TrimSpace(v))
		if err == nil && parsed.GreaterThan(decimal.Zero) {
			return uint(parsed.IntPart())
		}
	}
	return 0
}

func orderSnapshotStringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		return decimal.NewFromFloat(v).Round(2).StringFixed(2)
	case int:
		return decimal.NewFromInt(int64(v)).Round(2).StringFixed(2)
	case int64:
		return decimal.NewFromInt(v).Round(2).StringFixed(2)
	case uint:
		return decimal.NewFromInt(int64(v)).Round(2).StringFixed(2)
	}
	return ""
}

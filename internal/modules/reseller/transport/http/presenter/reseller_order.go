package presenter

import (
	"time"

	resellermodule "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

type ResellerOrderResp struct {
	OrderNo      string       `json:"order_no"`
	Status       string       `json:"status"`
	Currency     string       `json:"currency"`
	TotalAmount  money.Amount `json:"total_amount"`
	BaseAmount   money.Amount `json:"base_amount"`
	ProfitAmount money.Amount `json:"profit_amount"`
	ProfitStatus string       `json:"profit_status"`
	Domain       string       `json:"domain"`
	BuyerLabel   string       `json:"buyer_label"`
	ItemsCount   int          `json:"items_count"`
	CreatedAt    time.Time    `json:"created_at"`
	PaidAt       *time.Time   `json:"paid_at,omitempty"`
}

type ResellerOrderItemResp struct {
	Title               jsonmap.JSON `json:"title"`
	SKUSnapshot         jsonmap.JSON `json:"sku_snapshot"`
	Quantity            int          `json:"quantity"`
	UnitPrice           money.Amount `json:"unit_price"`
	TotalPrice          money.Amount `json:"total_price"`
	BaseUnitAmount      string       `json:"base_unit_amount,omitempty"`
	ResellerUnitAmount  string       `json:"reseller_unit_amount,omitempty"`
	BaseTotalAmount     string       `json:"base_total_amount,omitempty"`
	ResellerTotalAmount string       `json:"reseller_total_amount,omitempty"`
	ProfitAmount        string       `json:"profit_amount,omitempty"`
}

type ResellerOrderDetailResp struct {
	ResellerOrderResp
	Items []ResellerOrderItemResp `json:"items"`
}

type ResellerOrderStatsResp struct {
	Total      int64            `json:"total"`
	ByStatus   map[string]int64 `json:"by_status"`
	ByCurrency map[string]int64 `json:"by_currency"`
}

func NewResellerOrderResp(row resellermodule.OrderListItem) ResellerOrderResp {
	return ResellerOrderResp{
		OrderNo:      row.OrderNo,
		Status:       row.Status,
		Currency:     row.Currency,
		TotalAmount:  row.TotalAmount,
		BaseAmount:   row.BaseAmount,
		ProfitAmount: row.ProfitAmount,
		ProfitStatus: row.ProfitStatus,
		Domain:       row.Domain,
		BuyerLabel:   row.BuyerLabel,
		ItemsCount:   row.ItemsCount,
		CreatedAt:    row.CreatedAt,
		PaidAt:       row.PaidAt,
	}
}

func NewResellerOrderRespList(rows []resellermodule.OrderListItem) []ResellerOrderResp {
	out := make([]ResellerOrderResp, 0, len(rows))
	for i := range rows {
		out = append(out, NewResellerOrderResp(rows[i]))
	}
	return out
}

func NewResellerOrderDetailResp(row *resellermodule.OrderDetail) ResellerOrderDetailResp {
	if row == nil {
		return ResellerOrderDetailResp{}
	}
	resp := ResellerOrderDetailResp{ResellerOrderResp: NewResellerOrderResp(row.OrderListItem)}
	for i := range row.Items {
		item := row.Items[i]
		resp.Items = append(resp.Items, ResellerOrderItemResp{
			Title:               item.Title,
			SKUSnapshot:         item.SKUSnapshot,
			Quantity:            item.Quantity,
			UnitPrice:           item.UnitPrice,
			TotalPrice:          item.TotalPrice,
			BaseUnitAmount:      item.BaseUnitAmount,
			ResellerUnitAmount:  item.ResellerUnitAmount,
			BaseTotalAmount:     item.BaseTotalAmount,
			ResellerTotalAmount: item.ResellerTotalAmount,
			ProfitAmount:        item.ProfitAmount,
		})
	}
	return resp
}

func NewResellerOrderStatsResp(stats resellermodule.OrderStats) ResellerOrderStatsResp {
	return ResellerOrderStatsResp{
		Total:      stats.Total,
		ByStatus:   stats.ByStatus,
		ByCurrency: stats.ByCurrency,
	}
}

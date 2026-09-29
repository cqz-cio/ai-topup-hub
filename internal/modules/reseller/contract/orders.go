package contract

import (
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

const (
	ProfitStatusCredited    = "credited"
	ProfitStatusPending     = "pending"
	ProfitStatusUnavailable = "unavailable"
)

// OrderListInput 分销订单列表/统计查询输入。
type OrderListInput struct {
	Page        int
	PageSize    int
	Status      string
	OrderNo     string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	PaidFrom    *time.Time
	PaidTo      *time.Time
}

// OrderListItem 分销商视角订单列表项。
type OrderListItem struct {
	OrderNo      string
	Status       string
	Currency     string
	TotalAmount  money.Amount
	BaseAmount   money.Amount
	ProfitAmount money.Amount
	ProfitStatus string
	Domain       string
	BuyerLabel   string
	ItemsCount   int
	CreatedAt    time.Time
	PaidAt       *time.Time
}

// OrderItemDetail 分销商视角订单明细行。
type OrderItemDetail struct {
	Title               jsonmap.JSON
	SKUSnapshot         jsonmap.JSON
	Quantity            int
	UnitPrice           money.Amount
	TotalPrice          money.Amount
	BaseUnitAmount      string
	ResellerUnitAmount  string
	BaseTotalAmount     string
	ResellerTotalAmount string
	ProfitAmount        string
}

// OrderDetail 分销商视角订单详情。
type OrderDetail struct {
	OrderListItem
	Items []OrderItemDetail
}

// OrderStats 分销商视角订单统计。
type OrderStats struct {
	Total      int64
	ByStatus   map[string]int64
	ByCurrency map[string]int64
}

// OrderSnapshotListFilter 分销订单快照持久化过滤条件。
type OrderSnapshotListFilter struct {
	Page        int
	PageSize    int
	ResellerID  uint
	Status      string
	OrderNo     string
	Domain      string
	Currency    string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	PaidFrom    *time.Time
	PaidTo      *time.Time
}

// OrderSnapshotRow 聚合订单快照、订单展示字段、商品行和账务流水。
type OrderSnapshotRow struct {
	Snapshot      resellerdomain.OrderSnapshot
	Order         orderdomain.Order
	Items         []orderdomain.OrderItem
	LedgerEntries []resellerdomain.LedgerEntry
	BuyerEmail    string
}

// OrderStatsRow 分销商视角销售订单统计聚合行。
type OrderStatsRow struct {
	Total      int64
	ByStatus   map[string]int64
	ByCurrency map[string]int64
}

// OrderQueryStore 是分销销售订单只读查询用例所需的最小持久化端口。
type OrderQueryStore interface {
	GetProfileByUserID(userID uint) (*resellerdomain.Profile, error)
	GetProfileByID(id uint) (*resellerdomain.Profile, error)
	ListOrderSnapshotsByReseller(filter OrderSnapshotListFilter) ([]OrderSnapshotRow, int64, error)
	StatsOrderSnapshotsByReseller(filter OrderSnapshotListFilter) (OrderStatsRow, error)
	GetOrderSnapshotByResellerOrderNo(resellerID uint, orderNo string) (*OrderSnapshotRow, error)
}

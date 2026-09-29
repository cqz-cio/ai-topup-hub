package contract

import (
	"time"

	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// Repository is the persistence port consumed by the dashboard use cases.
type Repository interface {
	GetOverview(startAt, endAt time.Time) (OverviewRow, error)
	GetPaymentOrderAlertCounts(startAt, endAt time.Time) (PaymentOrderAlertCountsRow, error)
	GetOrderTrends(startAt, endAt time.Time) ([]OrderTrendRow, error)
	GetPaymentTrends(startAt, endAt time.Time) ([]PaymentTrendRow, error)
	GetProfitOverview(startAt, endAt time.Time) (ProfitOverviewRow, error)
	GetProfitTrends(startAt, endAt time.Time) ([]ProfitTrendRow, error)
	GetStockStats(lowStockThreshold int64) (StockStatsRow, error)
	GetInventoryAlertItems(lowStockThreshold int64) ([]InventoryAlertRow, error)
	GetTopProducts(startAt, endAt time.Time, limit int) ([]ProductRankingRow, error)
	GetTopChannels(startAt, endAt time.Time, limit int) ([]ChannelRankingRow, error)
	GetTotalUserBalance() (float64, error)
}

// SettingReader keeps the dashboard core independent from settings persistence.
type SettingReader interface {
	GetDashboardSetting() (settingsstorefront.DashboardSetting, error)
}

type OverviewRow struct {
	OrdersTotal          int64
	PaidOrders           int64
	CompletedOrders      int64
	PendingPaymentOrders int64
	ProcessingOrders     int64
	GMVPaid              float64
	PaymentsTotal        int64
	PaymentsSuccess      int64
	PaymentsFailed       int64
	NewUsers             int64
	ActiveProducts       int64
	Currency             string
}

type PaymentOrderAlertCountsRow struct {
	PendingPaymentOrders int64
	PaymentsFailed       int64
}

type OrderTrendRow struct {
	Day         string
	OrdersTotal int64
	OrdersPaid  int64
}

type PaymentTrendRow struct {
	Day             string
	PaymentsSuccess int64
	PaymentsFailed  int64
	GMVPaid         float64
}

type ProfitOverviewRow struct {
	TotalRevenue float64
	TotalCost    float64
	RefundedCost float64
	PaymentFee   float64
}

type ProfitTrendRow struct {
	Day          string
	Revenue      float64
	Cost         float64
	RefundedCost float64
	PaymentFee   float64
}

type StockStatsRow struct {
	OutOfStockProducts   int64
	LowStockProducts     int64
	OutOfStockSKUs       int64
	LowStockSKUs         int64
	AutoAvailableSecrets int64
	ManualAvailableUnits int64
}

type InventoryAlertRow struct {
	ProductID         uint
	SKUID             uint
	ProductTitleJSON  jsonmap.JSON
	SKUCode           string
	SKUSpecValuesJSON jsonmap.JSON
	FulfillmentType   string
	AlertType         string
	AvailableStock    int64
}

type ProductRankingRow struct {
	ProductID         uint
	SKUID             uint         `gorm:"column:sku_id"`
	SKUCode           string       `gorm:"column:sku_code"`
	SKUSpecValuesJSON jsonmap.JSON `gorm:"column:sku_spec_values_json;type:json"`
	Title             string
	PaidOrders        int64
	Quantity          int64
	PaidAmount        float64
	TotalCost         float64
}

type ChannelRankingRow struct {
	ChannelID     uint
	ChannelName   string
	ProviderType  string
	ChannelType   string
	SuccessCount  int64
	FailedCount   int64
	SuccessAmount float64
}

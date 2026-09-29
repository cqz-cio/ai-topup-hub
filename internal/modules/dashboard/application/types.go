package application

import "github.com/dujiao-next/internal/shared/jsonmap"

type OverviewResponse struct {
	Range    string      `json:"range"`
	From     string      `json:"from"`
	To       string      `json:"to"`
	Timezone string      `json:"timezone"`
	Currency string      `json:"currency,omitempty"`
	KPI      KPI         `json:"kpi"`
	Funnel   Funnel      `json:"funnel"`
	Alerts   []AlertItem `json:"alerts"`
}

type KPI struct {
	OrdersTotal          int64  `json:"orders_total"`
	PaidOrders           int64  `json:"paid_orders"`
	CompletedOrders      int64  `json:"completed_orders"`
	PendingPaymentOrders int64  `json:"pending_payment_orders"`
	ProcessingOrders     int64  `json:"processing_orders"`
	GMVPaid              string `json:"gmv_paid"`
	TotalCost            string `json:"total_cost"`
	PaymentFee           string `json:"payment_fee"`
	TotalProfit          string `json:"total_profit"`
	ProfitMargin         string `json:"profit_margin"`
	PaymentsTotal        int64  `json:"payments_total"`
	PaymentsSuccess      int64  `json:"payments_success"`
	PaymentsFailed       int64  `json:"payments_failed"`
	PaymentSuccessRate   string `json:"payment_success_rate"`
	NewUsers             int64  `json:"new_users"`
	ActiveProducts       int64  `json:"active_products"`
	OutOfStockProducts   int64  `json:"out_of_stock_products"`
	LowStockProducts     int64  `json:"low_stock_products"`
	OutOfStockSKUs       int64  `json:"out_of_stock_skus"`
	LowStockSKUs         int64  `json:"low_stock_skus"`
	AutoAvailableSecrets int64  `json:"auto_available_secrets"`
	ManualAvailableUnits int64  `json:"manual_available_units"`
	TotalUserBalance     string `json:"total_user_balance"`
}

type Funnel struct {
	OrdersCreated         int64  `json:"orders_created"`
	PaymentsCreated       int64  `json:"payments_created"`
	PaymentsSuccess       int64  `json:"payments_success"`
	OrdersPaid            int64  `json:"orders_paid"`
	OrdersCompleted       int64  `json:"orders_completed"`
	PaymentConversionRate string `json:"payment_conversion_rate"`
	CompletionRate        string `json:"completion_rate"`
}

type AlertItem struct {
	Type  string `json:"type"`
	Level string `json:"level"`
	Value int64  `json:"value"`
}

type TrendResponse struct {
	Range    string       `json:"range"`
	From     string       `json:"from"`
	To       string       `json:"to"`
	Timezone string       `json:"timezone"`
	Points   []TrendPoint `json:"points"`
}

type TrendPoint struct {
	Date            string `json:"date"`
	OrdersTotal     int64  `json:"orders_total"`
	OrdersPaid      int64  `json:"orders_paid"`
	PaymentsSuccess int64  `json:"payments_success"`
	PaymentsFailed  int64  `json:"payments_failed"`
	GMVPaid         string `json:"gmv_paid"`
	Profit          string `json:"profit"`
}

type RankingsResponse struct {
	Range       string           `json:"range"`
	From        string           `json:"from"`
	To          string           `json:"to"`
	Timezone    string           `json:"timezone"`
	TopProducts []ProductRanking `json:"top_products"`
	TopChannels []ChannelRanking `json:"top_channels"`
}

type ProductRanking struct {
	ProductID     uint         `json:"product_id"`
	SKUID         uint         `json:"sku_id,omitempty"`
	SKUCode       string       `json:"sku_code,omitempty"`
	SKUSpecValues jsonmap.JSON `json:"sku_spec_values,omitempty"`
	Title         string       `json:"title"`
	PaidOrders    int64        `json:"paid_orders"`
	Quantity      int64        `json:"quantity"`
	PaidAmount    string       `json:"paid_amount"`
	TotalCost     string       `json:"total_cost"`
	Profit        string       `json:"profit"`
}

type ChannelRanking struct {
	ChannelID     uint   `json:"channel_id"`
	ChannelName   string `json:"channel_name"`
	ProviderType  string `json:"provider_type"`
	ChannelType   string `json:"channel_type"`
	SuccessCount  int64  `json:"success_count"`
	FailedCount   int64  `json:"failed_count"`
	SuccessAmount string `json:"success_amount"`
	SuccessRate   string `json:"success_rate"`
}

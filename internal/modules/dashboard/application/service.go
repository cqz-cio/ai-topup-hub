package application

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	dashboardcontract "github.com/dujiao-next/internal/modules/dashboard/contract"
	reportingapp "github.com/dujiao-next/internal/modules/reporting/application"
	reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"
	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
)

const dashboardCacheTTL = 45 * time.Second

// Service 仪表盘服务
// 说明：聚合后台首页核心经营数据。
type Service struct {
	repo           dashboardcontract.Repository
	settingService dashboardcontract.SettingReader
}

// NewService 创建仪表盘服务
func NewService(repo dashboardcontract.Repository, settingService dashboardcontract.SettingReader) *Service {
	return &Service{repo: repo, settingService: settingService}
}

// GetOverview 获取仪表盘总览
func (s *Service) GetOverview(ctx context.Context, input reportingdomain.Query) (*OverviewResponse, error) {
	if s == nil || s.repo == nil {
		return &OverviewResponse{}, nil
	}

	window, err := reportingapp.Resolve(input, time.Now())
	if err != nil {
		return nil, err
	}

	setting := s.loadSetting()

	cacheKey := fmt.Sprintf("dashboard:overview:%s:%d:%d:%s:%d:%d:%d:%d:%t",
		window.Range,
		window.StartAt.Unix(),
		window.EndAt.Unix(),
		window.Timezone,
		setting.Alert.LowStockThreshold,
		setting.Alert.OutOfStockProductsThreshold,
		setting.Alert.PendingPaymentOrdersThreshold,
		setting.Alert.PaymentsFailedThreshold,
		setting.Accounting.RefundReversesCost,
	)
	if !input.ForceRefresh {
		var cached OverviewResponse
		hit, cacheErr := cache.GetJSON(ctx, cacheKey, &cached)
		if cacheErr == nil && hit {
			return &cached, nil
		}
	}

	overview, err := s.repo.GetOverview(window.StartAt, window.EndAt)
	if err != nil {
		return nil, err
	}
	profitOverview, err := s.repo.GetProfitOverview(window.StartAt, window.EndAt)
	if err != nil {
		return nil, err
	}
	stockStats, err := s.repo.GetStockStats(setting.Alert.LowStockThreshold)
	if err != nil {
		return nil, err
	}
	totalUserBalance, err := s.repo.GetTotalUserBalance()
	if err != nil {
		return nil, err
	}

	totalCost := effectiveDashboardCost(profitOverview.TotalCost, profitOverview.RefundedCost, setting.Accounting.RefundReversesCost) + profitOverview.PaymentFee
	totalProfit := profitOverview.TotalRevenue - totalCost
	profitMargin := 0.0
	if profitOverview.TotalRevenue > 0 {
		profitMargin = totalProfit / profitOverview.TotalRevenue * 100
	}

	paymentSuccessRate := 0.0
	if overview.PaymentsTotal > 0 {
		paymentSuccessRate = float64(overview.PaymentsSuccess) / float64(overview.PaymentsTotal) * 100
	}

	paymentConversionRate := 0.0
	if overview.OrdersTotal > 0 {
		paymentConversionRate = float64(overview.PaidOrders) / float64(overview.OrdersTotal) * 100
	}

	completionRate := 0.0
	if overview.PaidOrders > 0 {
		completionRate = float64(overview.CompletedOrders) / float64(overview.PaidOrders) * 100
	}

	response := &OverviewResponse{
		Range:    window.Range,
		From:     window.StartAt.Format(time.RFC3339),
		To:       window.EndAt.Add(-time.Second).Format(time.RFC3339),
		Timezone: window.Timezone,
		Currency: strings.ToUpper(strings.TrimSpace(overview.Currency)),
		KPI: KPI{
			OrdersTotal:          overview.OrdersTotal,
			PaidOrders:           overview.PaidOrders,
			CompletedOrders:      overview.CompletedOrders,
			PendingPaymentOrders: overview.PendingPaymentOrders,
			ProcessingOrders:     overview.ProcessingOrders,
			GMVPaid:              formatMoneyValue(overview.GMVPaid),
			TotalCost:            formatMoneyValue(totalCost),
			PaymentFee:           formatMoneyValue(profitOverview.PaymentFee),
			TotalProfit:          formatMoneyValue(totalProfit),
			ProfitMargin:         formatPercentValue(profitMargin),
			PaymentsTotal:        overview.PaymentsTotal,
			PaymentsSuccess:      overview.PaymentsSuccess,
			PaymentsFailed:       overview.PaymentsFailed,
			PaymentSuccessRate:   formatPercentValue(paymentSuccessRate),
			NewUsers:             overview.NewUsers,
			ActiveProducts:       overview.ActiveProducts,
			OutOfStockProducts:   stockStats.OutOfStockProducts,
			LowStockProducts:     stockStats.LowStockProducts,
			OutOfStockSKUs:       stockStats.OutOfStockSKUs,
			LowStockSKUs:         stockStats.LowStockSKUs,
			AutoAvailableSecrets: stockStats.AutoAvailableSecrets,
			ManualAvailableUnits: stockStats.ManualAvailableUnits,
			TotalUserBalance:     formatMoneyValue(totalUserBalance),
		},
		Funnel: Funnel{
			OrdersCreated:         overview.OrdersTotal,
			PaymentsCreated:       overview.PaymentsTotal,
			PaymentsSuccess:       overview.PaymentsSuccess,
			OrdersPaid:            overview.PaidOrders,
			OrdersCompleted:       overview.CompletedOrders,
			PaymentConversionRate: formatPercentValue(paymentConversionRate),
			CompletionRate:        formatPercentValue(completionRate),
		},
		Alerts: buildDashboardAlerts(overview, stockStats, setting.Alert),
	}

	_ = cache.SetJSON(ctx, cacheKey, response, dashboardCacheTTL)
	return response, nil
}

// LoadDashboardAlertSetting 获取仪表盘告警配置
func (s *Service) LoadDashboardAlertSetting() settingsstorefront.DashboardAlertSetting {
	return s.loadSetting().Alert
}

// GetInventoryAlertItems 获取库存异常明细
func (s *Service) GetInventoryAlertItems(_ context.Context, lowStockThreshold int64) ([]dashboardcontract.InventoryAlertRow, error) {
	if s == nil || s.repo == nil {
		return []dashboardcontract.InventoryAlertRow{}, nil
	}
	return s.repo.GetInventoryAlertItems(lowStockThreshold)
}

// GetPaymentOrderAlertCounts 获取支付订单告警计数
func (s *Service) GetPaymentOrderAlertCounts(_ context.Context, startAt, endAt time.Time) (dashboardcontract.PaymentOrderAlertCountsRow, error) {
	if s == nil || s.repo == nil {
		return dashboardcontract.PaymentOrderAlertCountsRow{}, nil
	}
	return s.repo.GetPaymentOrderAlertCounts(startAt, endAt)
}

// GetTrends 获取仪表盘趋势
func (s *Service) GetTrends(ctx context.Context, input reportingdomain.Query) (*TrendResponse, error) {
	if s == nil || s.repo == nil {
		return &TrendResponse{}, nil
	}

	window, err := reportingapp.Resolve(input, time.Now())
	if err != nil {
		return nil, err
	}

	setting := s.loadSetting()
	cacheKey := fmt.Sprintf("dashboard:trends:%s:%d:%d:%s:%t", window.Range, window.StartAt.Unix(), window.EndAt.Unix(), window.Timezone, setting.Accounting.RefundReversesCost)
	if !input.ForceRefresh {
		var cached TrendResponse
		hit, cacheErr := cache.GetJSON(ctx, cacheKey, &cached)
		if cacheErr == nil && hit {
			return &cached, nil
		}
	}

	orderRows, err := s.repo.GetOrderTrends(window.StartAt, window.EndAt)
	if err != nil {
		return nil, err
	}
	paymentRows, err := s.repo.GetPaymentTrends(window.StartAt, window.EndAt)
	if err != nil {
		return nil, err
	}
	profitRows, err := s.repo.GetProfitTrends(window.StartAt, window.EndAt)
	if err != nil {
		return nil, err
	}

	orderMap := make(map[string]dashboardcontract.OrderTrendRow, len(orderRows))
	for _, item := range orderRows {
		orderMap[item.Day] = item
	}
	paymentMap := make(map[string]dashboardcontract.PaymentTrendRow, len(paymentRows))
	for _, item := range paymentRows {
		paymentMap[item.Day] = item
	}
	profitMap := make(map[string]dashboardcontract.ProfitTrendRow, len(profitRows))
	for _, item := range profitRows {
		profitMap[item.Day] = item
	}

	points := make([]TrendPoint, 0)
	for cursor := time.Date(window.StartAt.Year(), window.StartAt.Month(), window.StartAt.Day(), 0, 0, 0, 0, window.StartAt.Location()); cursor.Before(window.EndAt); cursor = cursor.AddDate(0, 0, 1) {
		day := cursor.Format("2006-01-02")
		orderItem := orderMap[day]
		paymentItem := paymentMap[day]
		profitItem := profitMap[day]
		dayCost := effectiveDashboardCost(profitItem.Cost, profitItem.RefundedCost, setting.Accounting.RefundReversesCost) + profitItem.PaymentFee
		dayProfit := profitItem.Revenue - dayCost
		points = append(points, TrendPoint{
			Date:            day,
			OrdersTotal:     orderItem.OrdersTotal,
			OrdersPaid:      orderItem.OrdersPaid,
			PaymentsSuccess: paymentItem.PaymentsSuccess,
			PaymentsFailed:  paymentItem.PaymentsFailed,
			GMVPaid:         formatMoneyValue(paymentItem.GMVPaid),
			Profit:          formatMoneyValue(dayProfit),
		})
	}

	response := &TrendResponse{
		Range:    window.Range,
		From:     window.StartAt.Format(time.RFC3339),
		To:       window.EndAt.Add(-time.Second).Format(time.RFC3339),
		Timezone: window.Timezone,
		Points:   points,
	}

	_ = cache.SetJSON(ctx, cacheKey, response, dashboardCacheTTL)
	return response, nil
}

// GetRankings 获取仪表盘排行榜
func (s *Service) GetRankings(ctx context.Context, input reportingdomain.Query) (*RankingsResponse, error) {
	if s == nil || s.repo == nil {
		return &RankingsResponse{}, nil
	}

	window, err := reportingapp.Resolve(input, time.Now())
	if err != nil {
		return nil, err
	}

	setting := s.loadSetting()

	cacheKey := fmt.Sprintf("dashboard:rankings:%s:%d:%d:%s:%d:%d",
		window.Range,
		window.StartAt.Unix(),
		window.EndAt.Unix(),
		window.Timezone,
		setting.Ranking.TopProductsLimit,
		setting.Ranking.TopChannelsLimit,
	)
	if !input.ForceRefresh {
		var cached RankingsResponse
		hit, cacheErr := cache.GetJSON(ctx, cacheKey, &cached)
		if cacheErr == nil && hit {
			return &cached, nil
		}
	}

	productRows, err := s.repo.GetTopProducts(window.StartAt, window.EndAt, setting.Ranking.TopProductsLimit)
	if err != nil {
		return nil, err
	}
	channelRows, err := s.repo.GetTopChannels(window.StartAt, window.EndAt, setting.Ranking.TopChannelsLimit)
	if err != nil {
		return nil, err
	}

	products := make([]ProductRanking, 0, len(productRows))
	for _, item := range productRows {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = "-"
		}
		products = append(products, ProductRanking{
			ProductID:     item.ProductID,
			SKUID:         item.SKUID,
			SKUCode:       item.SKUCode,
			SKUSpecValues: item.SKUSpecValuesJSON,
			Title:         title,
			PaidOrders:    item.PaidOrders,
			Quantity:      item.Quantity,
			PaidAmount:    formatMoneyValue(item.PaidAmount),
			TotalCost:     formatMoneyValue(item.TotalCost),
			Profit:        formatMoneyValue(item.PaidAmount - item.TotalCost),
		})
	}

	channels := make([]ChannelRanking, 0, len(channelRows))
	for _, item := range channelRows {
		total := item.SuccessCount + item.FailedCount
		rate := 0.0
		if total > 0 {
			rate = float64(item.SuccessCount) / float64(total) * 100
		}
		channels = append(channels, ChannelRanking{
			ChannelID:     item.ChannelID,
			ChannelName:   strings.TrimSpace(item.ChannelName),
			ProviderType:  strings.TrimSpace(item.ProviderType),
			ChannelType:   strings.TrimSpace(item.ChannelType),
			SuccessCount:  item.SuccessCount,
			FailedCount:   item.FailedCount,
			SuccessAmount: formatMoneyValue(item.SuccessAmount),
			SuccessRate:   formatPercentValue(rate),
		})
	}

	response := &RankingsResponse{
		Range:       window.Range,
		From:        window.StartAt.Format(time.RFC3339),
		To:          window.EndAt.Add(-time.Second).Format(time.RFC3339),
		Timezone:    window.Timezone,
		TopProducts: products,
		TopChannels: channels,
	}

	_ = cache.SetJSON(ctx, cacheKey, response, dashboardCacheTTL)
	return response, nil
}

func (s *Service) loadSetting() settingsstorefront.DashboardSetting {
	fallback := settingsstorefront.DefaultDashboardSetting()
	if s == nil || s.settingService == nil {
		return fallback
	}
	setting, err := s.settingService.GetDashboardSetting()
	if err != nil {
		return fallback
	}
	return settingsstorefront.NormalizeDashboardSetting(setting)
}

func formatMoneyValue(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

func formatPercentValue(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

func effectiveDashboardCost(totalCost, refundedCost float64, refundReversesCost bool) float64 {
	if !refundReversesCost {
		return totalCost
	}
	result := totalCost - refundedCost
	if math.Abs(result) < 0.000001 {
		return 0
	}
	return result
}

func buildDashboardAlerts(overview dashboardcontract.OverviewRow, stockStats dashboardcontract.StockStatsRow, alertSetting settingsstorefront.DashboardAlertSetting) []AlertItem {
	alerts := make([]AlertItem, 0, 4)
	if stockStats.OutOfStockProducts >= alertSetting.OutOfStockProductsThreshold {
		alerts = append(alerts, AlertItem{Type: "out_of_stock_products", Level: "error", Value: stockStats.OutOfStockProducts})
	}
	if stockStats.LowStockProducts > 0 {
		alerts = append(alerts, AlertItem{Type: "low_stock_products", Level: "warning", Value: stockStats.LowStockProducts})
	}
	if overview.PendingPaymentOrders >= alertSetting.PendingPaymentOrdersThreshold {
		alerts = append(alerts, AlertItem{Type: "pending_payment_orders", Level: "warning", Value: overview.PendingPaymentOrders})
	}
	if overview.PaymentsFailed >= alertSetting.PaymentsFailedThreshold {
		alerts = append(alerts, AlertItem{Type: "payments_failed", Level: "warning", Value: overview.PaymentsFailed})
	}
	return alerts
}

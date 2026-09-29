package contract

import (
	"strings"
	"time"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

const (
	ProfitBlockOwner          = "self_dealing_owner"
	ProfitBlockRelatedAccount = "self_dealing_related_account"
)

// OrderPricingContext 分销下单定价上下文。
type OrderPricingContext struct {
	ResellerID        uint
	Domain            string
	Currency          string
	ResellerUserID    uint
	BuyerUserID       uint
	BaseAmount        decimal.Decimal
	ResellerAmount    decimal.Decimal
	ProfitAmount      decimal.Decimal
	EffectiveProfit   decimal.Decimal
	ProfitEligible    bool
	ProfitBlockReason string
	Items             []OrderPricingItem
	PricingSnapshot   jsonmap.JSON
	RiskSnapshot      jsonmap.JSON
}

// OrderPricingItem 分销下单定价明细行。
type OrderPricingItem struct {
	ProductID           uint
	SKUID               uint
	Quantity            int
	ChildOrderID        uint
	BaseUnitAmount      decimal.Decimal
	ResellerUnitAmount  decimal.Decimal
	BaseTotalAmount     decimal.Decimal
	ResellerTotalAmount decimal.Decimal
	ProfitAmount        decimal.Decimal
	PricingMode         string
	RuleSource          string
	SettingID           *uint
	OrderID             uint
	OrderItemID         uint
}

// DisplayPriceResult 分销站商品展示价结果。
type DisplayPriceResult struct {
	Visible      bool
	ProductID    uint
	DisplaySKUID uint
	DisplayPrice money.Amount
	SKUPrices    map[uint]money.Amount
	HiddenSKUIDs map[uint]bool
}

// DisplayPricingBatch 分销站批量展示定价所需配置。
type DisplayPricingBatch struct {
	Tenant            TenantContext
	Profile           *resellerdomain.Profile
	SettingsByProduct map[uint][]resellerdomain.ProductSetting
}

// SettingKey 商品/SKU 配置索引键。
type SettingKey struct {
	ProductID uint
	SKUID     uint
}

// BindCreatedOrderItem 绑定落库后的子订单与订单行 ID，并刷新定价快照。
func (ctx *OrderPricingContext) BindCreatedOrderItem(index int, childOrderID uint, orderItemID uint) {
	if ctx == nil || index < 0 || index >= len(ctx.Items) {
		return
	}
	ctx.Items[index].ChildOrderID = childOrderID
	ctx.Items[index].OrderID = childOrderID
	ctx.Items[index].OrderItemID = orderItemID
	ctx.PricingSnapshot = ctx.BuildPricingSnapshotJSON()
}

// BuildSnapshot 构造分销订单快照。
func (ctx *OrderPricingContext) BuildSnapshot(orderID uint, now time.Time) *resellerdomain.OrderSnapshot {
	if ctx == nil {
		return nil
	}
	for i := range ctx.Items {
		if ctx.Items[i].OrderID == 0 {
			ctx.Items[i].OrderID = ctx.Items[i].ChildOrderID
		}
	}
	ctx.PricingSnapshot = ctx.BuildPricingSnapshotJSON()
	return &resellerdomain.OrderSnapshot{
		OrderID:             orderID,
		ResellerID:          ctx.ResellerID,
		Domain:              ctx.Domain,
		Currency:            ctx.Currency,
		ResellerUserID:      ctx.ResellerUserID,
		BuyerUserID:         ctx.BuyerUserID,
		BaseAmount:          money.FromDecimal(ctx.BaseAmount),
		ResellerAmount:      money.FromDecimal(ctx.ResellerAmount),
		ProfitAmount:        money.FromDecimal(ctx.ProfitAmount),
		ProfitEligible:      ctx.ProfitEligible,
		ProfitBlockReason:   ctx.ProfitBlockReason,
		PricingSnapshotJSON: ctx.PricingSnapshot,
		RiskSnapshotJSON:    ctx.RiskSnapshot,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

// BuildPricingSnapshotJSON 生成定价快照 JSON。
func (ctx *OrderPricingContext) BuildPricingSnapshotJSON() jsonmap.JSON {
	items := make([]interface{}, 0, len(ctx.Items))
	for _, item := range ctx.Items {
		entry := jsonmap.JSON{
			"product_id":            item.ProductID,
			"sku_id":                item.SKUID,
			"quantity":              item.Quantity,
			"child_order_id":        item.ChildOrderID,
			"base_unit_amount":      MoneyString(item.BaseUnitAmount),
			"reseller_unit_amount":  MoneyString(item.ResellerUnitAmount),
			"base_total_amount":     MoneyString(item.BaseTotalAmount),
			"reseller_total_amount": MoneyString(item.ResellerTotalAmount),
			"profit_amount":         MoneyString(item.ProfitAmount),
			"pricing_mode":          item.PricingMode,
			"rule_source":           item.RuleSource,
			"order_id":              item.OrderID,
			"order_item_id":         item.OrderItemID,
		}
		if item.SettingID != nil {
			entry["setting_id"] = *item.SettingID
		} else {
			entry["setting_id"] = nil
		}
		items = append(items, entry)
	}
	return jsonmap.JSON{
		"currency":        ctx.Currency,
		"base_amount":     MoneyString(ctx.BaseAmount),
		"reseller_amount": MoneyString(ctx.ResellerAmount),
		"profit_amount":   MoneyString(ctx.ProfitAmount),
		"items":           items,
	}
}

// BuildRiskSnapshotJSON 生成风控快照 JSON。
func (ctx *OrderPricingContext) BuildRiskSnapshotJSON() jsonmap.JSON {
	if ctx.RiskSnapshot != nil {
		return ctx.RiskSnapshot
	}
	return jsonmap.JSON{
		"buyer_user_id":       ctx.BuyerUserID,
		"reseller_user_id":    ctx.ResellerUserID,
		"profit_eligible":     ctx.ProfitEligible,
		"profit_block_reason": ctx.ProfitBlockReason,
	}
}

// MoneyString 将金额格式化为两位小数字符串。
func MoneyString(value decimal.Decimal) string {
	return value.Round(2).StringFixed(2)
}

// BuildSettingIndexes 按商品级/SKU 级拆分配置索引。
func BuildSettingIndexes(settings []resellerdomain.ProductSetting) (map[uint]*resellerdomain.ProductSetting, map[SettingKey]*resellerdomain.ProductSetting) {
	byProduct := make(map[uint]*resellerdomain.ProductSetting)
	bySKU := make(map[SettingKey]*resellerdomain.ProductSetting)
	for i := range settings {
		setting := settings[i]
		if setting.ProductID == 0 {
			continue
		}
		row := setting
		if setting.SKUID == 0 {
			byProduct[setting.ProductID] = &row
			continue
		}
		bySKU[SettingKey{ProductID: setting.ProductID, SKUID: setting.SKUID}] = &row
	}
	return byProduct, bySKU
}

// ApplySelfDealingRisk 根据买家与分销商关系标记利润是否可结算。
// relatedAccountMatch 由调用方完成关联账号查询后传入，避免用例依赖仓储。
func ApplySelfDealingRisk(ctx *OrderPricingContext, profile *resellerdomain.Profile, relatedAccountMatch bool) {
	if ctx == nil || profile == nil {
		return
	}
	ownerMatch := false
	relatedMatch := false
	if ctx.BuyerUserID > 0 && ctx.BuyerUserID == profile.UserID {
		ownerMatch = true
		ctx.ProfitEligible = false
		ctx.ProfitBlockReason = ProfitBlockOwner
	} else if relatedAccountMatch {
		relatedMatch = true
		ctx.ProfitEligible = false
		ctx.ProfitBlockReason = ProfitBlockRelatedAccount
	}
	ctx.RiskSnapshot = jsonmap.JSON{
		"buyer_user_id":         ctx.BuyerUserID,
		"reseller_user_id":      ctx.ResellerUserID,
		"profit_eligible":       ctx.ProfitEligible,
		"profit_block_reason":   ctx.ProfitBlockReason,
		"guest_buyer":           ctx.BuyerUserID == 0,
		"self_dealing_deferred": "same_contact_and_risk_detected_account_linking",
		"self_dealing": jsonmap.JSON{
			"owner_match":           ownerMatch,
			"related_account_match": relatedMatch,
		},
	}
}

// SnapshotDomain 从租户上下文提取快照域名。
func SnapshotDomain(tenant TenantContext) string {
	if host := strings.TrimSpace(tenant.PrimaryDomain); host != "" {
		return NormalizeHost(host)
	}
	return NormalizeHost(tenant.Host)
}

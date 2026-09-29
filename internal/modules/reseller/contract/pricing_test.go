package contract

import (
	"testing"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/shopspring/decimal"
)

func TestApplySelfDealingRiskOwnerMatch(t *testing.T) {
	ctx := &OrderPricingContext{
		ResellerUserID: 9,
		BuyerUserID:    9,
		ProfitEligible: true,
	}
	ApplySelfDealingRisk(ctx, &resellerdomain.Profile{UserID: 9}, false)
	if ctx.ProfitEligible || ctx.ProfitBlockReason != ProfitBlockOwner {
		t.Fatalf("unexpected risk: eligible=%v reason=%s", ctx.ProfitEligible, ctx.ProfitBlockReason)
	}
}

func TestBuildSettingIndexes(t *testing.T) {
	byProduct, bySKU := BuildSettingIndexes([]resellerdomain.ProductSetting{
		{ProductID: 1, SKUID: 0, IsListed: true},
		{ProductID: 1, SKUID: 2, IsListed: false},
	})
	if byProduct[1] == nil || !byProduct[1].IsListed {
		t.Fatalf("expected product setting")
	}
	if bySKU[SettingKey{ProductID: 1, SKUID: 2}] == nil || bySKU[SettingKey{ProductID: 1, SKUID: 2}].IsListed {
		t.Fatalf("expected sku setting hidden")
	}
}

func TestMoneyString(t *testing.T) {
	if got := MoneyString(decimal.RequireFromString("1.2")); got != "1.20" {
		t.Fatalf("unexpected money string: %s", got)
	}
}

package application

import (
	"testing"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/upstream"

	"github.com/shopspring/decimal"
)

func TestConvertUpstreamWholesalePricesRemapsUpstreamSKUScope(t *testing.T) {
	tiers := convertUpstreamWholesalePrices(productdomain.WholesalePriceTiers{
		{SKUID: 201, MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
	}, decimal.NewFromInt(1), decimal.Zero, "none", buildUpstreamWholesaleSKUIndex(
		[]productdomain.ProductSKU{{ID: 11, SKUCode: "SKU-A"}},
		[]upstream.UpstreamSKU{{ID: 201, SKUCode: "SKU-A"}},
		nil,
	))

	if len(tiers) != 1 {
		t.Fatalf("expected 1 tier, got %d", len(tiers))
	}
	if tiers[0].SKUID != 11 || tiers[0].SKUCode != "SKU-A" {
		t.Fatalf("expected upstream SKU scope to be remapped, got %+v", tiers[0])
	}
}

func TestConvertUpstreamWholesalePricesDropsUnmappedUpstreamSKUID(t *testing.T) {
	tiers := convertUpstreamWholesalePrices(productdomain.WholesalePriceTiers{
		{SKUID: 201, MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
	}, decimal.NewFromInt(1), decimal.Zero, "none")

	if len(tiers) != 0 {
		t.Fatalf("expected unmapped upstream sku_id tier to be dropped, got %+v", tiers)
	}
}

package integrationtest

import (
	"errors"
	"strconv"
	"testing"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
	productwrite "github.com/dujiao-next/internal/modules/catalog/product/application/write"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestNormalizeWholesalePriceInputsSortsTiers(t *testing.T) {
	tiers, err := productdomain.NormalizeWholesalePrices([]productdomain.WholesalePriceInput{
		{MinQuantity: 10, UnitPrice: decimal.NewFromInt(70)},
		{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
	})
	if err != nil {
		t.Fatalf("normalizeWholesalePriceInputs returned error: %v", err)
	}
	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d", len(tiers))
	}
	if tiers[0].MinQuantity != 5 || tiers[0].UnitPrice.String() != "80.00" {
		t.Fatalf("unexpected first tier: %+v", tiers[0])
	}
	if tiers[1].MinQuantity != 10 || tiers[1].UnitPrice.String() != "70.00" {
		t.Fatalf("unexpected second tier: %+v", tiers[1])
	}
}

func TestNormalizeWholesalePriceInputsRejectsInvalidTiers(t *testing.T) {
	tests := []struct {
		name   string
		inputs []productdomain.WholesalePriceInput
	}{
		{
			name:   "zero quantity",
			inputs: []productdomain.WholesalePriceInput{{MinQuantity: 0, UnitPrice: decimal.NewFromInt(80)}},
		},
		{
			name:   "zero price",
			inputs: []productdomain.WholesalePriceInput{{MinQuantity: 5, UnitPrice: decimal.Zero}},
		},
		{
			name: "duplicate quantity",
			inputs: []productdomain.WholesalePriceInput{
				{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
				{MinQuantity: 5, UnitPrice: decimal.NewFromInt(70)},
			},
		},
		{
			name: "higher tier more expensive",
			inputs: []productdomain.WholesalePriceInput{
				{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
				{MinQuantity: 10, UnitPrice: decimal.NewFromInt(90)},
			},
		},
		{
			name: "higher tier equal price",
			inputs: []productdomain.WholesalePriceInput{
				{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
				{MinQuantity: 10, UnitPrice: decimal.NewFromInt(80)},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := productdomain.NormalizeWholesalePrices(tc.inputs)
			if !errors.Is(err, productdomain.ErrWholesalePriceInvalid) {
				t.Fatalf("expected productdomain.ErrWholesalePriceInvalid, got %v", err)
			}
		})
	}
}

func TestNormalizeWholesalePriceInputsRejectsDuplicateCanonicalSKUScope(t *testing.T) {
	_, err := productdomain.NormalizeWholesalePrices([]productdomain.WholesalePriceInput{
		{SKUID: 5, SKUCode: "SKU-A", MinQuantity: 10, UnitPrice: decimal.NewFromInt(80)},
		{SKUCode: "SKU-A", MinQuantity: 10, UnitPrice: decimal.NewFromInt(70)},
	})
	if !errors.Is(err, productdomain.ErrWholesalePriceInvalid) {
		t.Fatalf("expected productdomain.ErrWholesalePriceInvalid, got %v", err)
	}
}

func TestNormalizeWholesalePriceInputsRejectsNonDecreasingCanonicalSKUScope(t *testing.T) {
	_, err := productdomain.NormalizeWholesalePrices([]productdomain.WholesalePriceInput{
		{SKUID: 5, SKUCode: "SKU-A", MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
		{SKUCode: "SKU-A", MinQuantity: 10, UnitPrice: decimal.NewFromInt(90)},
	})
	if !errors.Is(err, productdomain.ErrWholesalePriceInvalid) {
		t.Fatalf("expected productdomain.ErrWholesalePriceInvalid, got %v", err)
	}
}

func TestNormalizeWholesalePriceInputsAllowsSameQuantityForDifferentSKUs(t *testing.T) {
	tiers, err := productdomain.NormalizeWholesalePrices([]productdomain.WholesalePriceInput{
		{SKUID: 2, SKUCode: "SKU-B", MinQuantity: 5, UnitPrice: decimal.NewFromInt(70)},
		{SKUID: 1, SKUCode: "SKU-A", MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
		{MinQuantity: 5, UnitPrice: decimal.NewFromInt(90)},
	})
	if err != nil {
		t.Fatalf("normalizeWholesalePriceInputs returned error: %v", err)
	}
	if len(tiers) != 3 {
		t.Fatalf("expected 3 tiers, got %d", len(tiers))
	}
	if tiers[0].SKUID != 0 || tiers[0].MinQuantity != 5 {
		t.Fatalf("expected universal tier first, got %+v", tiers[0])
	}
	if tiers[1].SKUID != 1 || tiers[1].SKUCode != "SKU-A" {
		t.Fatalf("expected SKU-A tier second, got %+v", tiers[1])
	}
	if tiers[2].SKUID != 2 || tiers[2].SKUCode != "SKU-B" {
		t.Fatalf("expected SKU-B tier third, got %+v", tiers[2])
	}
}

func TestResolveWholesaleUnitPriceMatchesBestTier(t *testing.T) {
	product := &productdomain.Product{
		WholesalePrices: productdomain.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
			{MinQuantity: 10, UnitPrice: money.FromDecimal(decimal.NewFromInt(70))},
		},
	}

	unitPrice, discount, matched := productdomain.ResolveWholesaleUnitPrice(product, decimal.NewFromInt(100), 12)
	if !matched {
		t.Fatalf("expected wholesale tier to match")
	}
	if !unitPrice.Equal(decimal.NewFromInt(70)) {
		t.Fatalf("expected unit price 70, got %s", unitPrice.String())
	}
	if !discount.Equal(decimal.NewFromInt(360)) {
		t.Fatalf("expected discount 360, got %s", discount.String())
	}
}

func TestResolveWholesaleUnitPriceDoesNotMatchBelowQuantity(t *testing.T) {
	product := &productdomain.Product{
		WholesalePrices: productdomain.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
		},
	}

	unitPrice, discount, matched := productdomain.ResolveWholesaleUnitPrice(product, decimal.NewFromInt(100), 4)
	if matched {
		t.Fatalf("expected no wholesale tier to match")
	}
	if !unitPrice.Equal(decimal.NewFromInt(100)) || !discount.IsZero() {
		t.Fatalf("unexpected price result: unit=%s discount=%s", unitPrice.String(), discount.String())
	}
}

func TestResolveWholesaleUnitPriceForSKUPrefersSpecificTier(t *testing.T) {
	product := &productdomain.Product{
		WholesalePrices: productdomain.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
			{SKUID: 11, SKUCode: "SKU-A", MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(70))},
		},
	}

	unitPrice, discount, matched := productdomain.ResolveWholesaleUnitPriceForSKU(product, decimal.NewFromInt(100), 11, "SKU-A", 12, 6)
	if !matched {
		t.Fatalf("expected SKU specific wholesale tier to match")
	}
	if !unitPrice.Equal(decimal.NewFromInt(70)) {
		t.Fatalf("expected unit price 70, got %s", unitPrice.String())
	}
	if !discount.Equal(decimal.NewFromInt(180)) {
		t.Fatalf("expected discount 180, got %s", discount.String())
	}
}

func TestResolveWholesaleUnitPriceForSKURequiresIDAndCodeToMatch(t *testing.T) {
	product := &productdomain.Product{
		WholesalePrices: productdomain.WholesalePriceTiers{
			{SKUID: 11, SKUCode: "SKU-A", MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(70))},
		},
	}

	if _, _, matched := productdomain.ResolveWholesaleUnitPriceForSKU(product, decimal.NewFromInt(100), 11, "SKU-B", 6, 6); matched {
		t.Fatalf("expected no match when sku_id matches but sku_code differs")
	}
	if _, _, matched := productdomain.ResolveWholesaleUnitPriceForSKU(product, decimal.NewFromInt(100), 12, "SKU-A", 6, 6); matched {
		t.Fatalf("expected no match when sku_code matches but sku_id differs")
	}
}

func TestResolveWholesaleUnitPriceForSKUDoesNotFallbackWhenSpecificTierExists(t *testing.T) {
	product := &productdomain.Product{
		WholesalePrices: productdomain.WholesalePriceTiers{
			{MinQuantity: 10, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
			{SKUID: 11, SKUCode: "SKU-A", MinQuantity: 10, UnitPrice: money.FromDecimal(decimal.NewFromInt(70))},
		},
	}

	unitPrice, discount, matched := productdomain.ResolveWholesaleUnitPriceForSKU(product, decimal.NewFromInt(100), 11, "SKU-A", 12, 6)
	if matched {
		t.Fatalf("expected no match because SKU specific threshold uses current SKU quantity")
	}
	if !unitPrice.Equal(decimal.NewFromInt(100)) || !discount.IsZero() {
		t.Fatalf("unexpected price result: unit=%s discount=%s", unitPrice.String(), discount.String())
	}
}

func TestResolveWholesaleUnitPriceForSKUUsesProductQuantityForUniversalTier(t *testing.T) {
	product := &productdomain.Product{
		WholesalePrices: productdomain.WholesalePriceTiers{
			{MinQuantity: 10, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
		},
	}

	unitPrice, discount, matched := productdomain.ResolveWholesaleUnitPriceForSKU(product, decimal.NewFromInt(100), 12, "SKU-B", 12, 6)
	if !matched {
		t.Fatalf("expected universal wholesale tier to match by product quantity")
	}
	if !unitPrice.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("expected unit price 80, got %s", unitPrice.String())
	}
	if !discount.Equal(decimal.NewFromInt(120)) {
		t.Fatalf("expected discount 120, got %s", discount.String())
	}
}

// TestResolveWholesaleUnitPricePicksCheapestTierForLegacyData 验证即便历史脏数据
// 存在非单调阶梯（高门槛档单价反而更高），选档也按单价最低者成交，避免「买更多反而更贵」。
func TestResolveWholesaleUnitPricePicksCheapestTierForLegacyData(t *testing.T) {
	product := &productdomain.Product{
		WholesalePrices: productdomain.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
			{MinQuantity: 10, UnitPrice: money.FromDecimal(decimal.NewFromInt(90))},
		},
	}

	// 购买 10 件时两档均满足门槛，应取更便宜的 80 而非门槛更高的 90。
	unitPrice, discount, matched := productdomain.ResolveWholesaleUnitPrice(product, decimal.NewFromInt(100), 10)
	if !matched {
		t.Fatalf("expected wholesale tier to match")
	}
	if !unitPrice.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("expected unit price 80, got %s", unitPrice.String())
	}
	if !discount.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("expected discount 200, got %s", discount.String())
	}
}

func TestResolveWholesaleUnitPriceIgnoresHigherTierPrice(t *testing.T) {
	product := &productdomain.Product{
		WholesalePrices: productdomain.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(120))},
		},
	}

	unitPrice, discount, matched := productdomain.ResolveWholesaleUnitPrice(product, decimal.NewFromInt(100), 5)
	if matched {
		t.Fatalf("expected higher wholesale price to be ignored")
	}
	if !unitPrice.Equal(decimal.NewFromInt(100)) || !discount.IsZero() {
		t.Fatalf("unexpected price result: unit=%s discount=%s", unitPrice.String(), discount.String())
	}
}

// TestProductServiceUpdateWholesalePricesOptionalSemantics 验证批发价的可选更新语义：
// Update 省略 wholesale_prices（nil）时保留原配置；传入空切片时显式清空。
func TestProductServiceUpdateWholesalePricesOptionalSemantics(t *testing.T) {
	svc, db := newProductServiceForTest(t)
	boolPtr := func(v bool) *bool { return &v }

	category := categorydomain.Category{
		Slug:     "wholesale-update-category",
		NameJSON: jsonmap.JSON{"zh-CN": "wholesale-update-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	created, err := svc.Write.Create(productwrite.CreateProductInput{
		CategoryID:      category.ID,
		Slug:            "wholesale-update",
		TitleJSON:       map[string]interface{}{"zh-CN": "wholesale-update"},
		PriceAmount:     decimal.NewFromInt(100),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		WholesalePrices: &[]productdomain.WholesalePriceInput{
			{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
		},
		IsActive: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	if len(created.WholesalePrices) != 1 {
		t.Fatalf("expected 1 wholesale tier on create, got %+v", created.WholesalePrices)
	}

	idStr := strconv.FormatUint(uint64(created.ID), 10)
	baseUpdate := func() productwrite.CreateProductInput {
		return productwrite.CreateProductInput{
			CategoryID:      category.ID,
			Slug:            created.Slug,
			TitleJSON:       map[string]interface{}{"zh-CN": "wholesale-update"},
			PriceAmount:     decimal.NewFromInt(100),
			PurchaseType:    constants.ProductPurchaseMember,
			FulfillmentType: constants.FulfillmentTypeAuto,
			IsActive:        boolPtr(true),
		}
	}

	// 省略字段（nil）：应保留原批发价。
	keep := baseUpdate()
	keep.WholesalePrices = nil
	updated, err := svc.Write.Update(idStr, keep)
	if err != nil {
		t.Fatalf("update without wholesale prices failed: %v", err)
	}
	if len(updated.WholesalePrices) != 1 || updated.WholesalePrices[0].UnitPrice.String() != "80.00" {
		t.Fatalf("expected wholesale prices kept when omitted, got %+v", updated.WholesalePrices)
	}

	// 传入空切片：显式清空。
	clear := baseUpdate()
	clear.WholesalePrices = &[]productdomain.WholesalePriceInput{}
	cleared, err := svc.Write.Update(idStr, clear)
	if err != nil {
		t.Fatalf("update with empty wholesale prices failed: %v", err)
	}
	if len(cleared.WholesalePrices) != 0 {
		t.Fatalf("expected wholesale prices cleared, got %+v", cleared.WholesalePrices)
	}
}

func TestProductServiceUpdateWholesalePricesOnlyTouchesWholesaleField(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	category := categorydomain.Category{
		Slug:     "wholesale-narrow-category",
		NameJSON: jsonmap.JSON{"zh-CN": "wholesale-narrow-category"},
		IsActive: true,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:       category.ID,
		Slug:             "wholesale-narrow-product",
		TitleJSON:        jsonmap.JSON{"zh-CN": "原商品名"},
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		CostPriceAmount:  money.FromDecimal(decimal.NewFromInt(30)),
		PurchaseType:     constants.ProductPurchaseMember,
		FulfillmentType:  constants.FulfillmentTypeManual,
		ManualStockTotal: 8,
		IsActive:         true,
		SortOrder:        9,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	updated, err := svc.Admin.UpdateWholesalePrices(strconv.FormatUint(uint64(product.ID), 10), []productdomain.WholesalePriceInput{
		{MinQuantity: 10, UnitPrice: decimal.RequireFromString("70.00")},
		{MinQuantity: 5, UnitPrice: decimal.RequireFromString("80.00")},
	})
	if err != nil {
		t.Fatalf("update wholesale prices failed: %v", err)
	}
	if len(updated.WholesalePrices) != 2 {
		t.Fatalf("expected 2 wholesale tiers, got %+v", updated.WholesalePrices)
	}
	if updated.WholesalePrices[0].MinQuantity != 5 || updated.WholesalePrices[0].UnitPrice.String() != "80.00" {
		t.Fatalf("expected first tier sorted as min=5 price=80.00, got %+v", updated.WholesalePrices[0])
	}
	if updated.WholesalePrices[1].MinQuantity != 10 || updated.WholesalePrices[1].UnitPrice.String() != "70.00" {
		t.Fatalf("expected second tier sorted as min=10 price=70.00, got %+v", updated.WholesalePrices[1])
	}

	var got productdomain.Product
	if err := db.First(&got, product.ID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if got.Slug != product.Slug || got.CategoryID != product.CategoryID || got.ManualStockTotal != product.ManualStockTotal || got.SortOrder != product.SortOrder || got.IsActive != product.IsActive {
		t.Fatalf("non-wholesale fields changed unexpectedly: got=%+v product=%+v", got, product)
	}
	if getTitle := got.TitleJSON["zh-CN"]; getTitle != "原商品名" {
		t.Fatalf("expected title to stay unchanged, got %v", getTitle)
	}
}

func TestProductServiceUpdateWholesalePricesClearsTiers(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	product := productdomain.Product{
		CategoryID:  1,
		Slug:        "wholesale-clear-product",
		TitleJSON:   jsonmap.JSON{"zh-CN": "wholesale-clear-product"},
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		WholesalePrices: productdomain.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
		},
		IsActive: true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	updated, err := svc.Admin.UpdateWholesalePrices(strconv.FormatUint(uint64(product.ID), 10), []productdomain.WholesalePriceInput{})
	if err != nil {
		t.Fatalf("clear wholesale prices failed: %v", err)
	}
	if len(updated.WholesalePrices) != 0 {
		t.Fatalf("expected wholesale prices cleared, got %+v", updated.WholesalePrices)
	}
}

func TestProductServiceUpdateWholesalePricesRejectsInvalidInputs(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	product := productdomain.Product{
		CategoryID:  1,
		Slug:        "wholesale-invalid-product",
		TitleJSON:   jsonmap.JSON{"zh-CN": "wholesale-invalid-product"},
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		WholesalePrices: productdomain.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
		},
		IsActive: true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	cases := []struct {
		name   string
		inputs []productdomain.WholesalePriceInput
	}{
		{name: "zero quantity", inputs: []productdomain.WholesalePriceInput{{MinQuantity: 0, UnitPrice: decimal.NewFromInt(80)}}},
		{name: "zero price", inputs: []productdomain.WholesalePriceInput{{MinQuantity: 5, UnitPrice: decimal.Zero}}},
		{name: "duplicate quantity", inputs: []productdomain.WholesalePriceInput{
			{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
			{MinQuantity: 5, UnitPrice: decimal.NewFromInt(70)},
		}},
	}

	idStr := strconv.FormatUint(uint64(product.ID), 10)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Admin.UpdateWholesalePrices(idStr, tc.inputs)
			if !errors.Is(err, productdomain.ErrWholesalePriceInvalid) {
				t.Fatalf("expected productdomain.ErrWholesalePriceInvalid, got %v", err)
			}
			var got productdomain.Product
			if err := db.First(&got, product.ID).Error; err != nil {
				t.Fatalf("reload product failed: %v", err)
			}
			if len(got.WholesalePrices) != 1 || got.WholesalePrices[0].UnitPrice.String() != "80.00" {
				t.Fatalf("expected existing wholesale prices preserved after invalid update, got %+v", got.WholesalePrices)
			}
		})
	}
}

func TestProductServiceUpdateWholesalePricesValidatesSKUBelonging(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	product := productdomain.Product{
		CategoryID:  1,
		Slug:        "wholesale-sku-owner-product",
		TitleJSON:   jsonmap.JSON{"zh-CN": "wholesale-sku-owner-product"},
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	skuA := productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     "SKU-A",
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		SortOrder:   1,
	}
	if err := db.Create(&skuA).Error; err != nil {
		t.Fatalf("create sku a failed: %v", err)
	}

	otherProduct := productdomain.Product{
		CategoryID:  1,
		Slug:        "wholesale-sku-owner-other",
		TitleJSON:   jsonmap.JSON{"zh-CN": "wholesale-sku-owner-other"},
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
	}
	if err := db.Create(&otherProduct).Error; err != nil {
		t.Fatalf("create other product failed: %v", err)
	}
	foreignSKU := productdomain.ProductSKU{
		ProductID:   otherProduct.ID,
		SKUCode:     "SKU-X",
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		SortOrder:   1,
	}
	if err := db.Create(&foreignSKU).Error; err != nil {
		t.Fatalf("create foreign sku failed: %v", err)
	}

	idStr := strconv.FormatUint(uint64(product.ID), 10)
	if _, err := svc.Admin.UpdateWholesalePrices(idStr, []productdomain.WholesalePriceInput{
		{SKUID: foreignSKU.ID, MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
	}); !errors.Is(err, productdomain.ErrWholesalePriceInvalid) {
		t.Fatalf("expected foreign sku_id to be rejected, got %v", err)
	}
	if _, err := svc.Admin.UpdateWholesalePrices(idStr, []productdomain.WholesalePriceInput{
		{SKUID: skuA.ID, SKUCode: "SKU-X", MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
	}); !errors.Is(err, productdomain.ErrWholesalePriceInvalid) {
		t.Fatalf("expected sku_id/sku_code mismatch to be rejected, got %v", err)
	}

	updated, err := svc.Admin.UpdateWholesalePrices(idStr, []productdomain.WholesalePriceInput{
		{SKUCode: "SKU-A", MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
	})
	if err != nil {
		t.Fatalf("update wholesale prices failed: %v", err)
	}
	if len(updated.WholesalePrices) != 1 {
		t.Fatalf("expected one tier, got %+v", updated.WholesalePrices)
	}
	if updated.WholesalePrices[0].SKUID != skuA.ID || updated.WholesalePrices[0].SKUCode != "SKU-A" {
		t.Fatalf("expected SKU code to be canonicalized, got %+v", updated.WholesalePrices[0])
	}
}

func TestProductServiceUpdateWholesalePricesReturnsNotFound(t *testing.T) {
	svc, _ := newProductServiceForTest(t)

	_, err := svc.Admin.UpdateWholesalePrices("999999", []productdomain.WholesalePriceInput{
		{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
	})
	if !errors.Is(err, productcontract.ErrNotFound) {
		t.Fatalf("expected productcontract.ErrNotFound, got %v", err)
	}
}

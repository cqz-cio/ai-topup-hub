package gormstore

import (
	"fmt"
	"testing"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestGetTopProductsIncludesChildOrderItems(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	now := time.Now()

	category := createDashboardCategory(t, db, "test-category")

	product := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "test-dashboard-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "测试商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	parentOrder := &orderdomain.Order{
		OrderNo:        "DJ-TEST-PARENT",
		UserID:         1,
		Status:         constants.OrderStatusPaid,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:      now,
	}
	if err := db.Create(parentOrder).Error; err != nil {
		t.Fatalf("create parent order failed: %v", err)
	}

	childOrder := &orderdomain.Order{
		OrderNo:        "DJ-TEST-PARENT-01",
		ParentID:       &parentOrder.ID,
		UserID:         1,
		Status:         constants.OrderStatusPaid,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:      now,
	}
	if err := db.Create(childOrder).Error; err != nil {
		t.Fatalf("create child order failed: %v", err)
	}

	orderItem := &orderdomain.OrderItem{
		OrderID:           childOrder.ID,
		ProductID:         product.ID,
		TitleJSON:         jsonmap.JSON{"zh-CN": "测试商品"},
		UnitPrice:         money.FromDecimal(decimal.NewFromInt(100)),
		Quantity:          2,
		TotalPrice:        money.FromDecimal(decimal.NewFromInt(200)),
		CouponDiscount:    money.FromDecimal(decimal.NewFromInt(10)),
		PromotionDiscount: money.FromDecimal(decimal.NewFromInt(20)),
		FulfillmentType:   constants.FulfillmentTypeManual,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(orderItem).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}

	rows, err := repo.GetTopProducts(now.Add(-time.Hour), now.Add(time.Hour), 5)
	if err != nil {
		t.Fatalf("get top products failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows len want 1 got %d", len(rows))
	}
	if rows[0].ProductID != product.ID {
		t.Fatalf("product id want %d got %d", product.ID, rows[0].ProductID)
	}
	if rows[0].PaidOrders != 1 {
		t.Fatalf("paid orders want 1 got %d", rows[0].PaidOrders)
	}
	if rows[0].Quantity != 2 {
		t.Fatalf("quantity want 2 got %d", rows[0].Quantity)
	}
	if rows[0].PaidAmount != 190 {
		t.Fatalf("paid amount want 190 got %.2f", rows[0].PaidAmount)
	}
}

func TestGetTopProductsGroupsBySKU(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	now := time.Now()
	category := createDashboardCategory(t, db, "sku-category")

	product := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "sku-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "订阅"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	skuA := &productdomain.ProductSKU{ProductID: product.ID, SKUCode: "MONTH-1", SpecValuesJSON: jsonmap.JSON{"zh-CN": "1个月"}, IsActive: true}
	skuB := &productdomain.ProductSKU{ProductID: product.ID, SKUCode: "MONTH-3", SpecValuesJSON: jsonmap.JSON{"zh-CN": "3个月"}, IsActive: true}
	if err := db.Create(skuA).Error; err != nil {
		t.Fatalf("create skuA failed: %v", err)
	}
	if err := db.Create(skuB).Error; err != nil {
		t.Fatalf("create skuB failed: %v", err)
	}

	for i, combo := range []struct {
		sku   *productdomain.ProductSKU
		qty   int
		total int64
	}{
		{skuA, 1, 50},
		{skuB, 2, 200},
		{skuB, 3, 300},
	} {
		order := &orderdomain.Order{
			OrderNo:        fmt.Sprintf("DJ-SKU-%d", i),
			UserID:         1,
			Status:         constants.OrderStatusPaid,
			Currency:       "CNY",
			OriginalAmount: money.FromDecimal(decimal.NewFromInt(combo.total)),
			TotalAmount:    money.FromDecimal(decimal.NewFromInt(combo.total)),
			CreatedAt:      now,
		}
		if err := db.Create(order).Error; err != nil {
			t.Fatalf("create order failed: %v", err)
		}
		if err := db.Create(&orderdomain.OrderItem{
			OrderID:         order.ID,
			ProductID:       product.ID,
			SKUID:           combo.sku.ID,
			TitleJSON:       product.TitleJSON,
			SKUSnapshotJSON: jsonmap.JSON{"sku_id": combo.sku.ID, "sku_code": combo.sku.SKUCode, "spec_values": combo.sku.SpecValuesJSON},
			UnitPrice:       money.FromDecimal(decimal.NewFromInt(combo.total / int64(combo.qty))),
			Quantity:        combo.qty,
			TotalPrice:      money.FromDecimal(decimal.NewFromInt(combo.total)),
			FulfillmentType: constants.FulfillmentTypeManual,
			CreatedAt:       now,
			UpdatedAt:       now,
		}).Error; err != nil {
			t.Fatalf("create item failed: %v", err)
		}
	}

	rows, err := repo.GetTopProducts(now.Add(-time.Hour), now.Add(time.Hour), 10)
	if err != nil {
		t.Fatalf("GetTopProducts failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expect 2 SKU rows, got %d", len(rows))
	}
	if rows[0].SKUID != skuB.ID {
		t.Fatalf("top sku want %d got %d", skuB.ID, rows[0].SKUID)
	}
	if rows[0].SKUCode != "MONTH-3" {
		t.Fatalf("sku code want MONTH-3 got %q", rows[0].SKUCode)
	}
	if rows[0].PaidOrders != 2 || rows[0].Quantity != 5 {
		t.Fatalf("skuB paid=%d qty=%d", rows[0].PaidOrders, rows[0].Quantity)
	}
	if rows[1].SKUID != skuA.ID || rows[1].SKUCode != "MONTH-1" {
		t.Fatalf("second row want skuA got %+v", rows[1])
	}
}

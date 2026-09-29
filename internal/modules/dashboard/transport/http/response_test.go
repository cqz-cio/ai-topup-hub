package dashboardhttp

import (
	"testing"

	dashboardcontract "github.com/dujiao-next/internal/modules/dashboard/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestMapInventoryAlertsPreservesLocalizedAndSKUFields(t *testing.T) {
	items := mapInventoryAlerts([]dashboardcontract.InventoryAlertRow{{
		ProductID:         7,
		SKUID:             9,
		ProductTitleJSON:  jsonmap.JSON{"zh-CN": "商品"},
		SKUCode:           "SKU-9",
		SKUSpecValuesJSON: jsonmap.JSON{"size": "L"},
		FulfillmentType:   "auto",
		AlertType:         "low_stock_products",
		AvailableStock:    2,
	}})
	if len(items) != 1 || items[0].ProductID != 7 || items[0].SKUID != 9 || items[0].SKUSpecValues["size"] != "L" {
		t.Fatalf("unexpected mapping: %+v", items)
	}
}

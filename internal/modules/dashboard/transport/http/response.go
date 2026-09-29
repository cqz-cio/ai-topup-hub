package dashboardhttp

import dashboardcontract "github.com/dujiao-next/internal/modules/dashboard/contract"

type inventoryAlertResponse struct {
	ProductID       uint                   `json:"product_id"`
	SKUID           uint                   `json:"sku_id,omitempty"`
	ProductTitle    map[string]interface{} `json:"product_title"`
	SKUCode         string                 `json:"sku_code,omitempty"`
	SKUSpecValues   map[string]interface{} `json:"sku_spec_values,omitempty"`
	FulfillmentType string                 `json:"fulfillment_type"`
	AlertType       string                 `json:"alert_type"`
	AvailableStock  int64                  `json:"available_stock"`
}

func mapInventoryAlerts(items []dashboardcontract.InventoryAlertRow) []inventoryAlertResponse {
	result := make([]inventoryAlertResponse, 0, len(items))
	for _, item := range items {
		row := inventoryAlertResponse{
			ProductID:       item.ProductID,
			SKUID:           item.SKUID,
			ProductTitle:    item.ProductTitleJSON,
			SKUCode:         item.SKUCode,
			FulfillmentType: item.FulfillmentType,
			AlertType:       item.AlertType,
			AvailableStock:  item.AvailableStock,
		}
		if item.SKUSpecValuesJSON != nil {
			row.SKUSpecValues = item.SKUSpecValuesJSON
		}
		result = append(result, row)
	}
	return result
}

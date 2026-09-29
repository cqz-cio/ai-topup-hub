package upstreamhttp

import (
	"testing"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
)

func TestComputeSKUStockUsesRemainingManualStockWithoutSubtractingLockedAgain(t *testing.T) {
	product := productdomain.Product{FulfillmentType: constants.FulfillmentTypeManual}
	sku := productdomain.ProductSKU{
		ManualStockTotal:  5,
		ManualStockLocked: 4,
	}

	status, quantity := computeSKUStock(product, sku)
	if quantity != 5 {
		t.Fatalf("manual_stock_total already represents remaining stock; quantity want 5 got %d", quantity)
	}
	if status != constants.ProductStockStatusLowStock {
		t.Fatalf("upstream API threshold should classify quantity 5 as low_stock, got %q", status)
	}
}

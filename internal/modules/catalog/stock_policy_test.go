package catalog

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func TestStockPolicyStatusKeepsContextThresholdsExplicit(t *testing.T) {
	tests := []struct {
		name     string
		policy   StockPolicy
		quantity int64
		want     string
	}{
		{name: "storefront unlimited", policy: StorefrontStockPolicy(), quantity: -1, want: constants.ProductStockStatusUnlimited},
		{name: "storefront out", policy: StorefrontStockPolicy(), quantity: 0, want: constants.ProductStockStatusOutOfStock},
		{name: "storefront low boundary", policy: StorefrontStockPolicy(), quantity: 5, want: constants.ProductStockStatusLowStock},
		{name: "storefront in stock", policy: StorefrontStockPolicy(), quantity: 6, want: constants.ProductStockStatusInStock},
		{name: "upstream low boundary", policy: UpstreamStockPolicy(), quantity: 20, want: constants.ProductStockStatusLowStock},
		{name: "upstream in stock", policy: UpstreamStockPolicy(), quantity: 21, want: constants.ProductStockStatusInStock},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.policy.Status(test.quantity); got != test.want {
				t.Fatalf("Status(%d) want %q got %q", test.quantity, test.want, got)
			}
		})
	}
}

func TestStorefrontDisplayModesAndRanges(t *testing.T) {
	rangeDisplay := StorefrontStockPolicy().Display(constants.ProductStockDisplayRange, "", 42)
	if rangeDisplay.Mode != constants.ProductStockDisplayRange || rangeDisplay.Display != constants.ProductStockDisplayRange21To50 {
		t.Fatalf("range display mismatch: %#v", rangeDisplay)
	}
	if rangeDisplay.RangeMin == nil || *rangeDisplay.RangeMin != 21 || rangeDisplay.RangeMax == nil || *rangeDisplay.RangeMax != 50 {
		t.Fatalf("range bounds mismatch: %#v", rangeDisplay)
	}
	if !rangeDisplay.QuantityHidden {
		t.Fatal("range mode must hide exact quantity")
	}

	statusDisplay := StorefrontStockPolicy().Display(constants.ProductStockDisplayStatus, "", 5)
	if statusDisplay.Display != constants.ProductStockStatusLowStock || !statusDisplay.QuantityHidden {
		t.Fatalf("status display mismatch: %#v", statusDisplay)
	}

	hiddenDisplay := StorefrontStockPolicy().Display(constants.ProductStockDisplayHidden, "", 6)
	if hiddenDisplay.Display != constants.ProductStockDisplayHidden || !hiddenDisplay.QuantityHidden {
		t.Fatalf("hidden display mismatch: %#v", hiddenDisplay)
	}

	exactDisplay := StorefrontStockPolicy().Display(constants.ProductStockDisplayExact, "", 6)
	if exactDisplay.Display != constants.ProductStockDisplayExact || exactDisplay.QuantityHidden {
		t.Fatalf("exact display mismatch: %#v", exactDisplay)
	}
}

func TestStockQuantityAndMasking(t *testing.T) {
	if got := StockQuantity(constants.FulfillmentTypeAuto, 9, 4); got != 9 {
		t.Fatalf("auto quantity want 9 got %d", got)
	}
	if got := StockQuantity(constants.FulfillmentTypeManual, 9, 4); got != 4 {
		t.Fatalf("manual quantity want 4 got %d", got)
	}
	if got := MaskStockInt(constants.ProductStockDisplayStatus, 42); got != 1 {
		t.Fatalf("masked positive stock want 1 got %d", got)
	}
	if got := MaskStockInt(constants.ProductStockDisplayStatus, constants.ManualStockUnlimited); got != constants.ManualStockUnlimited {
		t.Fatalf("masked unlimited stock want %d got %d", constants.ManualStockUnlimited, got)
	}
	if got := MaskStockInt64(constants.ProductStockDisplayRange, 42); got != 1 {
		t.Fatalf("masked int64 stock want 1 got %d", got)
	}
	if got := MaskSoldCount(constants.ProductStockDisplayHidden, 17); got != 0 {
		t.Fatalf("masked sold count want 0 got %d", got)
	}
}

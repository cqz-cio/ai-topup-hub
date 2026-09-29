package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChannelOrderHandlerIsSplitByResponsibility(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	handlerDirectory := filepath.Join(
		repositoryRoot,
		"internal", "modules", "channelapi", "transport", "http",
	)
	legacyPath := filepath.Join(handlerDirectory, "channel_order.go")
	if _, err := os.Stat(legacyPath); err == nil {
		t.Fatalf("channel_order.go must be replaced by responsibility-focused handler files")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat channel_order.go: %v", err)
	}

	expectedOwner := map[string]string{
		"PreviewOrder":                     "channel_order_create.go",
		"CreateOrder":                      "channel_order_create.go",
		"buildChannelOrderItems":           "channel_order_create.go",
		"buildChannelOrderPreviewResponse": "channel_order_create.go",
		"GetPaymentChannels":               "channel_order_payment.go",
		"GetLatestPayment":                 "channel_order_payment.go",
		"CreatePayment":                    "channel_order_payment.go",
		"GetPaymentDetail":                 "channel_order_payment.go",
		"buildChannelPaymentResponse":      "channel_order_payment.go",
		"GetOrderStatus":                   "channel_order_query.go",
		"GetOrderByOrderNo":                "channel_order_query.go",
		"CancelOrder":                      "channel_order_query.go",
		"ListOrders":                       "channel_order_query.go",
		"joinLocalizedInstructions":        "channel_order_query.go",
		"buildChannelOrderDetailResponse":  "channel_order_query.go",
		"channelOrderFulfillmentType":      "channel_order_query.go",
		"channelOrderPaidAmount":           "channel_order_view.go",
		"formatChannelNullableTime":        "channel_order_view.go",
		"channelLocalizedValue":            "channel_order_view.go",
		"channelLocaleValue":               "channel_order_view.go",
	}
	expectedTypeOwner := map[string]string{
		"channelOrderItemRequest": "channel_order_create.go",
		"previewOrderRequest":     "channel_order_create.go",
		"createOrderRequest":      "channel_order_create.go",
		"createPaymentRequest":    "channel_order_payment.go",
		"latestPaymentQuery":      "channel_order_payment.go",
		"orderListItem":           "channel_order_query.go",
		"cancelOrderRequest":      "channel_order_query.go",
	}

	files := []string{
		"channel_order_create.go",
		"channel_order_payment.go",
		"channel_order_query.go",
		"channel_order_view.go",
	}
	actualOwners := make(map[string][]string, len(expectedOwner))
	actualTypeOwners := make(map[string][]string, len(expectedTypeOwner))
	for _, file := range files {
		parsed := parseProductionGoFile(t, filepath.Join(handlerDirectory, file))
		for _, function := range declaredFunctionNames(parsed) {
			if _, tracked := expectedOwner[function]; tracked {
				actualOwners[function] = append(actualOwners[function], file)
			}
		}
		for _, typeName := range declaredTypeNames(parsed) {
			if _, tracked := expectedTypeOwner[typeName]; tracked {
				actualTypeOwners[typeName] = append(actualTypeOwners[typeName], file)
			}
		}
	}

	for function, wantFile := range expectedOwner {
		gotFiles := actualOwners[function]
		if len(gotFiles) != 1 || gotFiles[0] != wantFile {
			t.Errorf("%s ownership mismatch: want [%s], got %v", function, wantFile, gotFiles)
		}
	}
	for typeName, wantFile := range expectedTypeOwner {
		gotFiles := actualTypeOwners[typeName]
		if len(gotFiles) != 1 || gotFiles[0] != wantFile {
			t.Errorf("%s ownership mismatch: want [%s], got %v", typeName, wantFile, gotFiles)
		}
	}
	assertDirectoryGoFileBudget(t, handlerDirectory, 10)
	legacyFiles, err := filepath.Glob(filepath.Join(repositoryRoot, "internal", "http", "handlers", "channel", "*.go"))
	if err != nil {
		t.Fatalf("list legacy channel handlers: %v", err)
	}
	if len(legacyFiles) != 0 {
		t.Fatalf("legacy channel handlers must stay removed: %v", legacyFiles)
	}
	if _, err := os.Stat(filepath.Join(repositoryRoot, "internal", "router", "channel_adapter.go")); err == nil {
		t.Fatal("channel composition adapters belong in internal/bootstrap/channelapi, not internal/router")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy channel router adapter: %v", err)
	}
	wiringDirectory := filepath.Join(repositoryRoot, "internal", "bootstrap", "channelapi")
	if _, err := os.Stat(filepath.Join(wiringDirectory, "wiring.go")); err != nil {
		t.Fatalf("channel wiring missing: %v", err)
	}
	assertDirectoryGoFileBudget(t, wiringDirectory, 3)
}

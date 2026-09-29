package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrderAdminHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "order")
	domainRoot := filepath.Join(moduleRoot, "domain")
	contractRoot := filepath.Join(moduleRoot, "contract")
	applicationRoot := filepath.Join(moduleRoot, "application")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(repositoryRoot, "internal", "modules", "order", "transport", "http")
	presenterRoot := filepath.Join(repositoryRoot, "internal", "modules", "order", "transport", "presenter")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("order module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "order.go"), []string{"Order"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "order_item.go"), []string{"OrderItem"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "order_refund_record.go"), []string{"OrderRefundRecord"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "store.go"), []string{"Store", "Transaction"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "order_service.go"), []string{"OrderService", "OrderServiceOptions"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "order_service.go"), []string{"NewOrderService"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "order_store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "order_store.go"), []string{"New"})
	assertFileDeclaresTypes(t, filepath.Join(presenterRoot, "order.go"), []string{"OrderSummary", "OrderDetail"})
	assertDirectoryGoFileBudget(t, domainRoot, 5)
	assertDirectoryGoFileBudget(t, contractRoot, 4)
	assertDirectoryGoFileBudget(t, applicationRoot, 20)
	// 风控并发闸门保留独立测试文件，避免与租户、搜索等存储语义混杂。
	assertDirectoryGoFileBudget(t, storeRoot, 8)
	assertDirectoryGoFileBudget(t, presenterRoot, 3)
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/service")
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/repository")
	assertProductionImportsAbsent(t, applicationRoot, "gorm.io/gorm")

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{
		"RegisterAdminRoutes", "RegisterAdminRefundRoutes", "RegisterAdminRefundWriteRoutes",
		"RegisterUserReadRoutes", "RegisterUserCancelRoute", "RegisterUserPreviewRoute", "RegisterUserCreateRoute",
		"RegisterUserCreateAndPayRoute", "RegisterUserPaymentChannelsRoute",
		"RegisterGuestReadRoutes", "RegisterGuestPreviewRoute", "RegisterGuestCreateRoute",
		"RegisterGuestCreateAndPayRoute",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"AdminHandler", "OrderQuery", "UserDirectory", "CouponLookup", "PromotionLookup",
		"PaymentDirectory", "PaymentChannelDirectory", "OrderListFilter",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"NewAdminHandler", "AdminListOrders", "AdminGetOrder", "AdminUpdateOrderStatus",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_refund_handler.go"), []string{
		"AdminRefundHandler", "AdminRefundReader", "AdminRefundWriter", "AdminWalletRefunder",
		"OrderByIDLookup", "OrderStatusEmailEnqueuer", "AdminRefundListQuery", "AdminRefundItem",
		"AdminRefundToWalletInput", "AdminManualRefundInput",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_refund_handler.go"), []string{
		"NewAdminRefundHandler", "GetAdminOrderRefunds", "GetAdminOrderRefund",
		"AdminRefundOrderToWallet", "AdminManualRefundOrder",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_handler.go"), []string{
		"UserHandler", "UserOrderQuery", "PaymentChannelPolicy", "RefundRecordDirectory", "UserLookup",
		"UserOrderListFilter", "AvailablePaymentChannelFilter", "OrderPaymentChannelsRequest",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "user_handler.go"), []string{
		"NewUserHandler", "ListOrders", "OrderStats", "GetOrderByOrderNo", "DownloadFulfillment",
		"GetOrderPaymentChannels", "CancelOrder",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "guest_handler.go"), []string{
		"GuestHandler", "GuestOrderQuery",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "guest_handler.go"), []string{
		"NewGuestHandler", "ListGuestOrders", "GetGuestOrderByOrderNo", "DownloadGuestFulfillment",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "preview_handler.go"), []string{
		"PreviewHandler", "OrderPreviewService", "OrderPreview", "CreateOrderInput", "CreateGuestOrderInput",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "preview_handler.go"), []string{
		"NewPreviewHandler", "PreviewOrder", "PreviewGuestOrder",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "create_handler.go"), []string{
		"CreateHandler", "OrderCreateService", "GuestCreateCaptcha", "OrderPaymentCreator",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "create_handler.go"), []string{
		"NewCreateHandler", "CreateOrder", "CreateGuestOrder", "CreateOrderAndPay", "CreateGuestOrderAndPay",
	})
	assertDirectoryGoFileBudget(t, transportRoot, 7)

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "order_admin.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_order_refund.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "order.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "guest_order.go"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy admin order handler must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy admin order handler: %v", err)
		}
	}

	if _, err := os.Stat(filepath.Join(repositoryRoot, "internal", "router", "order_adapter.go")); err == nil {
		t.Fatal("order composition adapters belong in internal/bootstrap/order, not internal/router")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy order router adapter: %v", err)
	}
	wiringRoot := filepath.Join(repositoryRoot, "internal", "bootstrap", "order")
	for _, file := range []string{"wiring.go", "adapters.go"} {
		if _, err := os.Stat(filepath.Join(wiringRoot, file)); err != nil {
			t.Fatalf("order wiring file %s missing: %v", file, err)
		}
	}
	assertDirectoryGoFileBudget(t, wiringRoot, 4)
}

package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPaymentHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	transportRoot := filepath.Join(repositoryRoot, "internal", "modules", "payment", "transport", "http")

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{
		"RegisterGuestLatestRoute", "RegisterUserLatestRoute",
		"RegisterGuestWriteRoutes", "RegisterUserWriteRoutes",
		"RegisterAdminRoutes", "RegisterAdminChannelRoutes",
		"RegisterWebhookRoutes",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "latest_handler.go"), []string{
		"LatestHandler", "GuestOrderLookup", "UserOrderLookup", "PendingPaymentLookup",
		"LatestGuestPaymentQuery", "LatestPaymentQuery",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "latest_handler.go"), []string{
		"NewLatestHandler", "GetGuestLatestPayment", "GetLatestPayment",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "write_handler.go"), []string{
		"WriteHandler", "PaymentWriter", "CreatePaymentInput", "CreatePaymentResult", "CapturePaymentInput",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "write_handler.go"), []string{
		"NewWriteHandler", "CreatePayment", "CapturePayment", "CreateGuestPayment", "CaptureGuestPayment",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"AdminHandler", "AdminPaymentQuery", "AdminChannelLookup", "AdminOrderLookup", "AdminRechargeLookup",
		"AdminPaymentListFilter", "AdminPaymentItem",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"NewAdminHandler", "GetAdminPayments", "ExportAdminPayments", "GetAdminPayment",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_channel_handler.go"), []string{
		"AdminChannelHandler", "AdminChannelCatalog", "AdminChannelListFilter",
		"CreatePaymentChannelRequest", "UpdatePaymentChannelRequest",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_channel_handler.go"), []string{
		"NewAdminChannelHandler", "CreatePaymentChannel", "UpdatePaymentChannel",
		"DeletePaymentChannel", "GetPaymentChannel", "GetPaymentChannels", "TestWechatPayPublicKey",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "webhook_handler.go"), []string{
		"WebhookHandler", "PaymentWebhookService", "ExceptionAlerter", "WebhookCallbackInput",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "webhook_handler.go"), []string{
		"NewWebhookHandler", "PaypalWebhook", "StripeWebhook", "DujiaoPayWebhook",
	})
	assertDirectoryGoFileBudget(t, transportRoot, 9)

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "guest_payment.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "payment_webhook.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "error_mapping.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_payment.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_payment_channel.go"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy payment handler must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy payment handler: %v", err)
		}
	}

	if _, err := os.Stat(filepath.Join(repositoryRoot, "internal", "router", "payment_adapter.go")); err == nil {
		t.Fatal("payment composition adapters belong in internal/bootstrap/payment, not internal/router")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy payment router adapter: %v", err)
	}
	wiringRoot := filepath.Join(repositoryRoot, "internal", "bootstrap", "payment")
	for _, file := range []string{"wiring.go", "storefront.go", "admin_callbacks.go"} {
		if _, err := os.Stat(filepath.Join(wiringRoot, file)); err != nil {
			t.Fatalf("payment wiring file %s missing: %v", file, err)
		}
	}
	assertDirectoryGoFileBudget(t, wiringRoot, 4)
}

func TestPaymentCallbackHTTPLivesInFocusedTransportPackage(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	transportRoot := filepath.Join(repositoryRoot, "internal", "modules", "payment", "transport", "http", "callback")

	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "handler.go"), []string{
		"Handler", "Service", "PaymentLookup", "ChannelLookup", "ExceptionAlerter", "WechatWebhookInput",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "handler.go"), []string{
		"NewHandler", "PaymentCallback",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterRoutes"})
	assertDirectoryGoFileBudget(t, transportRoot, 8)

	legacyRoot := filepath.Join(repositoryRoot, "internal", "http", "handlers", "public")
	legacyFiles, err := filepath.Glob(filepath.Join(legacyRoot, "*.go"))
	if err != nil {
		t.Fatalf("list legacy public payment callback package: %v", err)
	}
	if len(legacyFiles) != 0 {
		t.Fatalf("legacy public payment callback Go files must stay removed: %v", legacyFiles)
	}
}

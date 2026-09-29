package architecture

import (
	"go/ast"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type packageFileBudget struct {
	production int
	total      int
}

// Test-only architecture assertions intentionally share one package so they can
// reuse AST helpers. Production packages have no file-budget exceptions.
var packageFileBudgetOverrides = map[string]packageFileBudget{
	"internal/architecture": {production: 0, total: 51},
}

// completedMigrationPaths are deleted compatibility-free entry points. Once a
// bounded context reaches this list, recreating its former horizontal package
// is an architecture regression rather than an allowed transitional change.
var completedMigrationPaths = []string{
	"internal/dto",
	"internal/http",
	"internal/integration",
	"internal/models",
	"internal/provider",
	"internal/repository",
	"internal/router",
	"internal/service",
	"internal/transport",
	"internal/wiring",
	"internal/worker",
	"internal/models/payment.go",
	"internal/models/payment_channel.go",
	"internal/repository/payment_repository.go",
	"internal/repository/payment_channel_repository.go",
	"internal/repository/payment_repository_test.go",
	"internal/service/payment_service.go",
	"internal/service/payment_service_available_channels.go",
	"internal/service/payment_service_available_channels_test.go",
	"internal/service/payment_service_callback.go",
	"internal/service/payment_service_callback_dispatch.go",
	"internal/service/payment_service_callback_wallet.go",
	"internal/service/payment_service_capture.go",
	"internal/service/payment_service_capture_test.go",
	"internal/service/payment_service_channel_rules.go",
	"internal/service/payment_service_context.go",
	"internal/service/payment_service_context_test.go",
	"internal/service/payment_service_create.go",
	"internal/service/payment_service_exchange_test.go",
	"internal/service/payment_service_gateway.go",
	"internal/service/payment_service_gateway_order_test.go",
	"internal/service/payment_service_notification_payload.go",
	"internal/service/payment_service_paypal_test.go",
	"internal/service/payment_service_provider.go",
	"internal/service/payment_service_query.go",
	"internal/service/payment_service_recharge.go",
	"internal/service/payment_service_recharge_expire.go",
	"internal/service/payment_service_return_query_test.go",
	"internal/service/payment_service_return_url_test.go",
	"internal/service/payment_service_rules.go",
	"internal/service/payment_service_stripe_test.go",
	"internal/service/payment_service_wallet_test.go",
	"internal/service/payment_service_webhook.go",
	"internal/service/payment_service_webhook_test.go",
	"internal/service/payment_service_wechat_test.go",
	"internal/service/payment_notification_payload_test.go",
	"internal/service/reseller_accounting_test.go",
	"internal/dto/payment.go",
	"internal/dto/payment_usdt.go",
	"internal/dto/payment_usdt_test.go",
	"internal/payment",
	"internal/transport/http/payment",
	"internal/wiring/payment",
	"internal/models/order.go",
	"internal/models/order_item.go",
	"internal/models/order_refund_record.go",
	"internal/models/fulfillment.go",
	"internal/repository/order_repository.go",
	"internal/repository/order_refund_record_repository.go",
	"internal/repository/fulfillment_repository.go",
	"internal/repository/admin_search_repository_test.go",
	"internal/repository/order_refund_record_repository_test.go",
	"internal/repository/order_reseller_scope_test.go",
	"internal/service/order_service.go",
	"internal/service/order_service_child.go",
	"internal/service/order_service_query.go",
	"internal/service/order_service_validate.go",
	"internal/service/order_status.go",
	"internal/service/order_status_email_queue.go",
	"internal/service/order_wallet_bridge.go",
	"internal/service/order_service_refund.go",
	"internal/service/order_refund_child_status.go",
	"internal/service/order_refund_wallet.go",
	"internal/service/manual_stock.go",
	"internal/service/reseller_pricing.go",
	"internal/service/fulfillment_service.go",
	"internal/service/order_reseller_snapshot_test.go",
	"internal/service/order_service_cancel_test.go",
	"internal/service/order_service_helpers_test.go",
	"internal/service/order_service_pricing_test.go",
	"internal/service/order_service_refund_test.go",
	"internal/service/order_service_status_test.go",
	"internal/service/order_status_email_queue_test.go",
	"internal/service/order_refund_wallet_test.go",
	"internal/service/order_wholesale_test.go",
	"internal/service/reseller_pricing_test.go",
	"internal/service/fulfillment_service_test.go",
	"internal/dto/order.go",
	"internal/dto/order_test.go",
	"internal/transport/http/order",
	"internal/transport/http/fulfillment",
	"internal/wiring/order",
	"internal/wiring/fulfillment",
	"internal/models/reseller.go",
	"internal/models/reseller_migrations.go",
	"internal/models/reseller_model_test.go",
	"internal/repository/reseller_repository.go",
	"internal/repository/reseller_repository_test.go",
	"internal/repository/reseller_profile_repository.go",
	"internal/repository/reseller_domain_repository.go",
	"internal/repository/reseller_site_config_repository.go",
	"internal/repository/reseller_pricing_read_repository.go",
	"internal/repository/reseller_pricing_repository_test.go",
	"internal/repository/reseller_product_setting_repository.go",
	"internal/repository/reseller_product_setting_repository_test.go",
	"internal/repository/reseller_order_snapshot_repository.go",
	"internal/repository/reseller_ledger_repository.go",
	"internal/repository/reseller_balance_repository.go",
	"internal/repository/reseller_withdraw_repository.go",
	"internal/repository/reseller_admin_repository.go",
	"internal/repository/reseller_operations_repository.go",
	"internal/repository/reseller_operations_repository_test.go",
	"internal/repository/reseller_accounting_repository_test.go",
	"internal/repository/reseller_postgres_integration_test.go",
	"internal/persistence/reseller",
	"internal/dto/reseller.go",
	"internal/dto/reseller_admin_profile.go",
	"internal/dto/reseller_order.go",
	"internal/dto/reseller_test.go",
	"internal/service/reseller_context.go",
	"internal/service/reseller_accounting_core.go",
	"internal/service/reseller_accounting_query.go",
	"internal/service/reseller_accounting_profit.go",
	"internal/service/reseller_accounting_refund.go",
	"internal/service/reseller_accounting_withdraw.go",
	"internal/service/reseller_accounting_ledger_store_adapter.go",
	"internal/service/reseller_accounting_withdraw_store_adapter.go",
	"internal/modules/reseller/accounting_balance.go",
	"internal/modules/reseller/accounting_profit.go",
	"internal/modules/reseller/accounting_query.go",
	"internal/modules/reseller/accounting_query_test.go",
	"internal/modules/reseller/accounting_refund.go",
	"internal/modules/reseller/accounting_types.go",
	"internal/modules/reseller/accounting_withdraw.go",
	"internal/modules/reseller/accounting_withdraw_test.go",
	"internal/modules/reseller/domain_resolver.go",
	"internal/modules/reseller/domain_resolver_test.go",
	"internal/modules/reseller/errors.go",
	"internal/modules/reseller/management.go",
	"internal/modules/reseller/operations.go",
	"internal/modules/reseller/operations_test.go",
	"internal/modules/reseller/operations_types.go",
	"internal/modules/reseller/order_query.go",
	"internal/modules/reseller/order_query_test.go",
	"internal/modules/reseller/order_types.go",
	"internal/modules/reseller/ports.go",
	"internal/modules/reseller/pricing_rules.go",
	"internal/modules/reseller/pricing_types.go",
	"internal/modules/reseller/pricing_types_test.go",
	"internal/modules/reseller/product_setting.go",
	"internal/modules/reseller/site_config.go",
	"internal/modules/reseller/tenant.go",
	"internal/modules/reseller/tenant_test.go",
	"internal/transport/http/reseller",
	"internal/wiring/reseller",
	"internal/models/wallet_account.go",
	"internal/models/wallet_transaction.go",
	"internal/models/wallet_recharge_order.go",
	"internal/repository/wallet_repository.go",
	"internal/repository/wallet_repository_impl.go",
	"internal/repository/wallet_repository_test.go",
	"internal/service/wallet_service.go",
	"internal/service/wallet_core.go",
	"internal/service/wallet_query.go",
	"internal/service/wallet_admin.go",
	"internal/service/wallet_credit.go",
	"internal/service/wallet_order.go",
	"internal/service/wallet_recharge.go",
	"internal/service/wallet_service_test.go",
	"internal/service/concurrency_sqlite_test.go",
	"internal/modules/wallet/errors.go",
	"internal/modules/wallet/ports.go",
	"internal/modules/wallet/query.go",
	"internal/modules/wallet/store",
	"internal/transport/http/wallet",
	"internal/wiring/wallet",
	"internal/models/procurement_order.go",
	"internal/repository/procurement_order_repository.go",
	"internal/http/handlers/admin/admin_procurement_order.go",
	"internal/service/procurement_order_service.go",
	"internal/service/procurement_order_create.go",
	"internal/service/procurement_order_submit.go",
	"internal/service/procurement_order_callback.go",
	"internal/service/procurement_order_poll.go",
	"internal/service/procurement_order_query.go",
	"internal/service/procurement_order_manual.go",
	"internal/service/procurement_order_lifecycle.go",
	"internal/modules/procurement/service.go",
	"internal/modules/procurement/create.go",
	"internal/modules/procurement/submit.go",
	"internal/modules/procurement/callback.go",
	"internal/modules/procurement/poll.go",
	"internal/modules/procurement/query.go",
	"internal/modules/procurement/manual.go",
	"internal/modules/procurement/rules_test.go",
	"internal/modules/procurement/store",
	"internal/modules/procurement/infrastructure/orderlifecycle",
	"internal/transport/http/procurement",
	"internal/integration/procurement",
	"internal/models/reconciliation.go",
	"internal/modules/reconciliation/service.go",
	"internal/modules/reconciliation/execute.go",
	"internal/modules/reconciliation/query.go",
	"internal/modules/reconciliation/service_test.go",
	"internal/modules/reconciliation/store",
	"internal/transport/http/reconciliation",
	"internal/modules/reporting/window.go",
	"internal/modules/reporting/window_test.go",
	"internal/modules/orderrisk/service.go",
	"internal/modules/orderrisk/service_test.go",
	"internal/http/handlers/shared/captcha_payload.go",
	"internal/models/setting.go",
	"internal/repository/setting_repository.go",
	"internal/service/setting_service.go",
	"internal/transport/http/captcha",
	"internal/transport/http/settings",
	"internal/wiring/captcha",
	"internal/wiring/settings",
	"internal/models/telegram_broadcast.go",
	"internal/repository/telegram_broadcast_repository.go",
	"internal/service/telegram_broadcast_service.go",
	"internal/transport/http/telegram/admin_broadcast_handler.go",
	"internal/wiring/telegram/broadcast.go",
	"internal/models/channel_client.go",
	"internal/repository/channel_client_repository.go",
	"internal/transport/http/channelclient",
	"internal/wiring/channelclient",
	"internal/wiring/telegram/channel_bot.go",
	"internal/service/telegram_auth_service.go",
	"internal/service/telegram_auth_service_test.go",
	"internal/service/telegram_oidc.go",
	"internal/service/telegram_oidc_test.go",
	"internal/models/email_verify_code.go",
	"internal/repository/email_verify_code_repository.go",
	"internal/models/user_oauth_identity.go",
	"internal/repository/user_oauth_identity_repository.go",
	"internal/repository/user_oauth_identity_repository_test.go",
	"internal/models/admin.go",
	"internal/models/init.go",
	"internal/models/init_test.go",
	"internal/repository/admin_repository.go",
	"internal/repository/admin_repository_test.go",
	"internal/models/money.go",
	"internal/models/user.go",
	"internal/repository/user_repository.go",
	"internal/repository/user_repository_test.go",
	"internal/service/totp_enable.go",
	"internal/service/totp_enable_test.go",
	"internal/service/recovery_codes.go",
	"internal/service/password_policy.go",
	"internal/service/jwt_parser.go",
	"internal/service/user_auth_service.go",
	"internal/service/user_auth_service_profile.go",
	"internal/service/user_auth_telegram_binding.go",
	"internal/service/user_auth_telegram_channel.go",
	"internal/service/user_auth_telegram_identity.go",
	"internal/service/user_auth_telegram_login.go",
	"internal/service/user_auth_telegram_oidc.go",
	"internal/service/user_auth_service_2fa_test.go",
	"internal/service/user_auth_service_channel_identity_test.go",
	"internal/service/user_auth_service_domain_policy_test.go",
	"internal/service/user_auth_service_email_mode_test.go",
	"internal/service/user_auth_service_oauth_miniapp_test.go",
	"internal/service/user_auth_service_oauth_test.go",
	"internal/service/user_auth_telegram_test_helpers_test.go",
	"internal/service/user_totp_service.go",
	"internal/service/user_totp_service_test.go",
	"internal/service/auth_service.go",
	"internal/service/auth_service_test.go",
	"internal/service/totp_service.go",
	"internal/service/totp_service_test.go",
	"internal/models/affiliate_profile.go",
	"internal/models/affiliate_click.go",
	"internal/models/affiliate_commission.go",
	"internal/models/affiliate_withdraw_request.go",
	"internal/repository/affiliate_repository.go",
	"internal/service/affiliate_service.go",
	"internal/service/affiliate_attribution.go",
	"internal/service/affiliate_commission.go",
	"internal/service/affiliate_profile.go",
	"internal/service/affiliate_query.go",
	"internal/service/affiliate_withdraw.go",
	"internal/service/affiliate_service_test.go",
	"internal/transport/http/affiliate",
	"internal/wiring/affiliate",
	"internal/modules/affiliate/errors.go",
	"internal/modules/affiliate/types.go",
	"internal/dto/affiliate.go",
	"internal/dto/affiliate_test.go",
	"internal/integration/affiliate",
	"internal/service/product_service.go",
	"internal/service/product_application_compat.go",
	"internal/repository/product_compat.go",
	"internal/repository/product_mapping_compat.go",
	"internal/service/product_mapping_service.go",
	"internal/service/product_mapping_service_test.go",
	"internal/models/category.go",
	"internal/modules/catalog/category_service.go",
	"internal/modules/catalog/store/gormstore/category_store.go",
	"internal/modules/catalog/store/gormstore/category_store_test.go",
	"internal/transport/http/catalog/admin_category_handler.go",
	"internal/integration/catalog/category_service_test.go",
	"internal/dto/product_test.go",
	"internal/models/product.go",
	"internal/models/product_sku.go",
	"internal/models/wholesale_price.go",
	"internal/modules/catalog/product/errors.go",
	"internal/modules/catalog/product/ports.go",
	"internal/dto/product.go",
	"internal/transport/http/catalog/admin_product_handler.go",
	"internal/transport/http/catalog/admin_product_handler_integration_test.go",
	"internal/transport/http/catalog/public_handler.go",
	"internal/transport/http/catalog/public_view.go",
	"internal/transport/http/catalog/public_stock.go",
	"internal/transport/http/catalog/public_price_integration_test.go",
	"internal/transport/http/catalog/public_price_test.go",
	"internal/transport/http/catalog/public_related_posts_test.go",
	"internal/transport/http/catalog/public_stock_test.go",
	"internal/transport/http/catalog/admin_product_mapping_handler.go",
	"internal/transport/http/catalog/admin_product_mapping_handler_integration_test.go",
	"internal/transport/http/catalog/routes.go",
	"internal/transport/http/catalog",
	"internal/wiring/catalog/factory.go",
	"internal/wiring/catalog/wiring.go",
	"internal/wiring/catalog",
	"internal/service/product_admin_test.go",
	"internal/service/product_create_test.go",
	"internal/service/product_query_test.go",
	"internal/service/product_reseller_public_test.go",
	"internal/service/product_sku_test.go",
	"internal/service/product_stock_test.go",
	"internal/service/product_test_helpers_test.go",
	"internal/service/product_update_test.go",
	"internal/service/product_wholesale_test.go",
	"internal/models/site_connection.go",
	"internal/service/site_connection_service.go",
	"internal/http/handlers/admin/admin_site_connection.go",
	"internal/modules/siteconnection/errors.go",
	"internal/modules/siteconnection/service.go",
	"internal/modules/siteconnection/types.go",
	"internal/repository/site_connection_repository.go",
	"internal/transport/http/siteconnection",
	"internal/integration/siteconnection",
	"internal/wiring/siteconnection",
	"internal/models/product_mapping.go",
	"internal/modules/catalog/mapping/batch_import.go",
	"internal/modules/catalog/mapping/batch_import_test.go",
	"internal/modules/catalog/mapping/import.go",
	"internal/modules/catalog/mapping/import_test.go",
	"internal/modules/catalog/mapping/markup.go",
	"internal/modules/catalog/mapping/pricing.go",
	"internal/modules/catalog/mapping/pricing_test.go",
	"internal/modules/catalog/mapping/service.go",
	"internal/modules/catalog/mapping/service_test.go",
	"internal/modules/catalog/mapping/sync.go",
	"internal/modules/catalog/mapping/sync_test.go",
	"internal/modules/catalog/mapping/store",
	"internal/models/member_level.go",
	"internal/models/member_level_price.go",
	"internal/modules/memberlevel/ports.go",
	"internal/modules/memberlevel/service.go",
	"internal/modules/memberlevel/store",
	"internal/transport/http/memberlevel",
	"internal/models/promotion.go",
	"internal/modules/promotion/admin_service.go",
	"internal/modules/promotion/errors.go",
	"internal/modules/promotion/ports.go",
	"internal/modules/promotion/service.go",
	"internal/modules/promotion/store",
	"internal/transport/http/promotion",
	"internal/models/coupon.go",
	"internal/models/coupon_usage.go",
	"internal/modules/coupon/admin_service.go",
	"internal/modules/coupon/errors.go",
	"internal/modules/coupon/ports.go",
	"internal/modules/coupon/service.go",
	"internal/modules/coupon/store",
	"internal/transport/http/coupon",
	"internal/models/gift_card.go",
	"internal/models/gift_card_batch.go",
	"internal/modules/giftcard/errors.go",
	"internal/modules/giftcard/export.go",
	"internal/modules/giftcard/generate.go",
	"internal/modules/giftcard/helpers.go",
	"internal/modules/giftcard/helpers_test.go",
	"internal/modules/giftcard/manage.go",
	"internal/modules/giftcard/ports.go",
	"internal/modules/giftcard/service.go",
	"internal/modules/giftcard/types.go",
	"internal/modules/giftcard/store",
	"internal/service/gift_card_service.go",
	"internal/service/gift_card_service_test.go",
	"internal/service/gift_card_store_adapter.go",
	"internal/dto/gift_card.go",
	"internal/transport/http/giftcard",
	"internal/integration/channel/giftcard_test.go",
	"internal/models/api_credential.go",
	"internal/modules/apicredential/ports.go",
	"internal/modules/apicredential/service.go",
	"internal/modules/apicredential/store",
	"internal/transport/http/apicredential",
	"internal/models/admin_login_log.go",
	"internal/models/user_login_log.go",
	"internal/models/authz_audit_log.go",
	"internal/modules/auditlog/authz_service.go",
	"internal/modules/auditlog/ports.go",
	"internal/modules/auditlog/service_test.go",
	"internal/modules/auditlog/store",
	"internal/modules/auditlog/user_login_service.go",
	"internal/transport/http/auditlog",
	"internal/dto/login_log.go",
	"internal/models/notification_log.go",
	"internal/modules/notification/alert.go",
	"internal/modules/notification/dedupe.go",
	"internal/modules/notification/errors.go",
	"internal/modules/notification/inventory_alert.go",
	"internal/modules/notification/log_service.go",
	"internal/modules/notification/log_service_test.go",
	"internal/modules/notification/order_format.go",
	"internal/modules/notification/payment_order_alert.go",
	"internal/modules/notification/ports.go",
	"internal/modules/notification/send.go",
	"internal/modules/notification/service.go",
	"internal/modules/notification/store",
	"internal/modules/notification/template.go",
	"internal/modules/notification/test_variables.go",
	"internal/transport/http/notification",
	"internal/models/post.go",
	"internal/models/post_category.go",
	"internal/models/banner.go",
	"internal/models/media.go",
	"internal/dto/post.go",
	"internal/dto/banner.go",
	"internal/dto/banner_test.go",
	"internal/modules/content/banner_service.go",
	"internal/modules/content/errors.go",
	"internal/modules/content/media_service.go",
	"internal/modules/content/ports.go",
	"internal/modules/content/post_category_service.go",
	"internal/modules/content/post_service.go",
	"internal/modules/content/query.go",
	"internal/modules/content/service_test.go",
	"internal/modules/content/system.go",
	"internal/modules/content/store",
	"internal/modules/content/filestore",
	"internal/transport/http/content",
	"internal/modules/dashboard/errors.go",
	"internal/modules/dashboard/ports.go",
	"internal/modules/dashboard/service.go",
	"internal/modules/dashboard/service_test.go",
	"internal/modules/dashboard/types.go",
	"internal/modules/dashboard/store",
	"internal/transport/http/dashboard",
	"internal/http/handlers/shared/reporting_query.go",
	"internal/http/handlers/shared/reporting_query_test.go",
	"internal/models/cart_item.go",
	"internal/repository/cart_repository.go",
	"internal/repository/cart_repository_test.go",
	"internal/modules/cart/service.go",
	"internal/dto/cart.go",
	"internal/transport/http/cart",
	"internal/wiring/cart",
	"internal/adgateway",
	"internal/modules/adproxy/service.go",
	"internal/modules/adproxy/types.go",
	"internal/transport/http/adproxy",
	"internal/wiring/adproxy",
	"internal/modules/compliance/service.go",
	"internal/modules/compliance/types.go",
	"internal/modules/compliance/errors.go",
	"internal/integration/compliance",
	"internal/transport/http/compliance",
	"internal/wiring/compliance",
	"internal/modules/sitemap/service.go",
	"internal/integration/sitemap",
	"internal/transport/http/sitemap",
	"internal/wiring/sitemap",
	"internal/modules/upload/service.go",
	"internal/modules/upload/service_test.go",
	"internal/transport/http/upload",
	"internal/wiring/upload",
	"internal/modules/captcha/service.go",
	"internal/modules/captcha/errors.go",
	"internal/bootstrap/captchahttp",
	"internal/modules/telegram/notify.go",
	"internal/modules/telegram/notify_test.go",
	"internal/modules/telegram/errors.go",
	"internal/transport/http/telegram",
	"internal/wiring/telegram",
	"internal/models/downstream_order_ref.go",
	"internal/repository/downstream_order_ref_repository.go",
	"internal/modules/downstreamcallback/service.go",
	"internal/integration/downstreamcallback",
	"internal/models/card_secret.go",
	"internal/models/card_secret_batch.go",
	"internal/repository/card_secret_compat.go",
	"internal/modules/cardsecret/service.go",
	"internal/modules/cardsecret/import.go",
	"internal/modules/cardsecret/manage.go",
	"internal/modules/cardsecret/export.go",
	"internal/modules/cardsecret/stats.go",
	"internal/modules/cardsecret/store",
	"internal/integration/cardsecret",
	"internal/transport/http/cardsecret",
}

func TestCompletedMigrationPathsStayDeleted(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	for _, relativePath := range completedMigrationPaths {
		t.Run(strings.ReplaceAll(relativePath, "/", "_"), func(t *testing.T) {
			absolutePath := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
			if _, err := os.Stat(absolutePath); err == nil {
				t.Fatalf("completed migration path was recreated: %s", relativePath)
			} else if !os.IsNotExist(err) {
				t.Fatalf("stat completed migration path %s: %v", relativePath, err)
			}
		})
	}
}

func TestSettingsModuleRootContainsNoGoFiles(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	settingsRoot := filepath.Join(repositoryRoot, "internal", "modules", "settings")
	production, total := countDirectGoFiles(t, settingsRoot)
	if production != 0 || total != 0 {
		t.Fatalf("settings module root must remain structural only, got production=%d total=%d", production, total)
	}
}

func TestSharedMoneyOwnsMonetaryValueObject(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moneyRoot := filepath.Join(repositoryRoot, "internal", "shared", "money")
	assertFileDeclaresTypes(t, filepath.Join(moneyRoot, "amount.go"), []string{"Amount"})
	assertFileDeclaresFunctions(t, filepath.Join(moneyRoot, "amount.go"), []string{"FromDecimal"})
	assertDirectoryGoFileBudget(t, moneyRoot, 2)
}

func TestSharedPasswordPolicyOwnsStrengthValidation(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	policyRoot := filepath.Join(repositoryRoot, "internal", "shared", "passwordpolicy")
	assertFileDeclaresTypes(t, filepath.Join(policyRoot, "policy.go"), []string{"Policy"})
	assertFileDeclaresFunctions(t, filepath.Join(policyRoot, "policy.go"), []string{"Validate"})
	assertDirectoryGoFileBudget(t, policyRoot, 2)
}

func TestAffiliateModuleOwnsDomainAndPersistence(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	domainRoot := filepath.Join(repositoryRoot, "internal", "modules", "affiliate", "domain")
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "profile.go"), []string{"Profile"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "click.go"), []string{"Click"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "commission.go"), []string{"Commission"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "withdraw_request.go"), []string{"WithdrawRequest"})
	assertDirectoryGoFileBudget(t, domainRoot, 5)

	storeRoot := filepath.Join(repositoryRoot, "internal", "modules", "affiliate", "infrastructure", "gormstore")
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertDirectoryGoFileBudget(t, storeRoot, 2)

	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "affiliate")
	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("affiliate module root must remain structural only, got production=%d total=%d", production, total)
	}
}

func TestDatabaseBootstrapIsSeparatedFromPlatformConnection(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	connectionRoot := filepath.Join(repositoryRoot, "internal", "platform", "database", "gormdb")
	migrationRoot := filepath.Join(repositoryRoot, "internal", "bootstrap", "database", "migrations")

	assertFileDeclaresTypes(t, filepath.Join(connectionRoot, "db.go"), []string{"DBPoolConfig"})
	assertFileDeclaresFunctions(t, filepath.Join(connectionRoot, "db.go"), []string{"InitDB"})
	assertFileDeclaresFunctions(t, filepath.Join(migrationRoot, "registry.go"), []string{"AutoMigrate"})
	assertDirectoryGoFileBudget(t, connectionRoot, 2)
	assertDirectoryGoFileBudget(t, migrationRoot, 4)
}

func TestNoNewCompatibilityOrLegacyProductionFiles(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	internalRoot := filepath.Join(repositoryRoot, "internal")

	err := filepath.WalkDir(internalRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		name := strings.ToLower(entry.Name())
		if !strings.Contains(name, "compat") && !strings.Contains(name, "legacy") && !strings.Contains(name, "facade") {
			return nil
		}

		relativePath, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		t.Errorf("compatibility or legacy production file is forbidden: %s", relativePath)
		return nil
	})
	if err != nil {
		t.Fatalf("inspect production Go files: %v", err)
	}
}

func TestProductionCodeContainsNoTypeAliases(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	internalRoot := filepath.Join(repositoryRoot, "internal")

	err := filepath.WalkDir(internalRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		parsed := parseProductionGoFile(t, path)
		ast.Inspect(parsed, func(node ast.Node) bool {
			typeSpec, ok := node.(*ast.TypeSpec)
			if ok && typeSpec.Assign.IsValid() {
				relativePath, err := filepath.Rel(repositoryRoot, path)
				if err != nil {
					t.Fatalf("resolve type alias path: %v", err)
				}
				t.Errorf(
					"production type alias %s is forbidden; depend on its owning package or declare an owned type: %s",
					typeSpec.Name.Name,
					filepath.ToSlash(relativePath),
				)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("inspect production Go aliases: %v", err)
	}
}

func TestGoPackagesStayWithinFileBudgets(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	internalRoot := filepath.Join(repositoryRoot, "internal")

	err := filepath.WalkDir(internalRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() || entry.Name() == "testdata" {
			return nil
		}

		production, total := countDirectGoFiles(t, path)
		if total == 0 {
			return nil
		}

		relativePath, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)

		budget := packageFileBudget{production: 12, total: 20}
		if override, ok := packageFileBudgetOverrides[relativePath]; ok {
			budget = override
		}

		if production > budget.production || total > budget.total {
			t.Errorf(
				"Go package %s exceeds its migration budget: production=%d/%d total=%d/%d",
				relativePath,
				production,
				budget.production,
				total,
				budget.total,
			)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect Go package budgets: %v", err)
	}
}

func countDirectGoFiles(t *testing.T, directory string) (production int, total int) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read %s: %v", directory, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		total++
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			production++
		}
	}
	return production, total
}

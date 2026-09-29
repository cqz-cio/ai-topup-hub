package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalletOwnsCompleteVerticalSlice(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "wallet")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	domainRoot := filepath.Join(moduleRoot, "domain")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")
	presenterRoot := filepath.Join(moduleRoot, "transport", "presenter")
	bootstrapRoot := filepath.Join(repositoryRoot, "internal", "bootstrap", "wallet")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("wallet module root must remain structural only, got production=%d total=%d", production, total)
	}

	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"Repository", "Transaction", "UnitOfWork", "UseCase",
	})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "types.go"), []string{
		"AccountListFilter", "TransactionListFilter", "RechargeListFilter",
		"RechargeInput", "AdjustBalanceInput", "CreditInput",
		"OrderBalanceInput", "OrderReleaseInput",
	})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "account.go"), []string{"Account"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "transaction.go"), []string{"Transaction"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "recharge_order.go"), []string{"RechargeOrder"})

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Options", "Service"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "query.go"), []string{
		"GetAccount", "ListTransactions", "ListRechargeOrdersAdmin",
		"ListUserRechargeOrders", "StatsUserRechargeOrders", "GetRechargeOrderByRechargeNo",
		"GetRechargeOrderByPaymentIDAndUser", "GetBalancesByUserIDs",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "admin.go"), []string{
		"Recharge", "AdminAdjustBalance",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "credit.go"), []string{
		"CreditInTransaction",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "order_balance.go"), []string{
		"ApplyOrderBalance", "ReleaseOrderBalance",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "recharge.go"), []string{
		"ApplyRechargePayment",
	})

	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "store.go"), []string{
		"New", "Bind", "UseTransaction", "WithinTransaction",
	})

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{
		"RegisterUserRoutes", "RegisterAdminRoutes", "RegisterChannelRoutes",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_handler.go"), []string{
		"WalletService", "PaymentService", "UserReader", "SiteCurrencyReader", "UserHandler",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"AdminWalletService", "AdminUserReader", "PaymentChannelReader", "PaymentReader",
		"AdminHandler", "AdminRechargeListFilter", "AdjustBalanceInput",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "channel_handler.go"), []string{
		"ChannelUserProvisioner", "ChannelHandler",
	})
	assertFileDeclaresTypes(t, filepath.Join(presenterRoot, "wallet.go"), []string{
		"WalletAccountResp", "WalletTransactionResp", "WalletRechargeResp",
		"WalletRechargePaymentPayload",
	})
	assertFileDeclaresFunctions(t, filepath.Join(presenterRoot, "wallet.go"), []string{
		"NewWalletAccountResp", "NewWalletTransactionResp", "NewWalletTransactionRespList",
		"NewWalletRechargeResp", "NewWalletRechargeRespList", "NewWalletRechargePaymentPayload",
	})
	assertFileDeclaresTypes(t, filepath.Join(bootstrapRoot, "handlers.go"), []string{"Handlers"})
	assertFileDeclaresFunctions(t, filepath.Join(bootstrapRoot, "handlers.go"), []string{"New"})

	assertDirectoryGoFileBudget(t, applicationRoot, 7)
	assertDirectoryGoFileBudget(t, contractRoot, 4)
	assertDirectoryGoFileBudget(t, domainRoot, 4)
	assertDirectoryGoFileBudget(t, storeRoot, 2)
	assertDirectoryGoFileBudget(t, transportRoot, 6)
	assertDirectoryGoFileBudget(t, presenterRoot, 2)
	assertDirectoryGoFileBudget(t, bootstrapRoot, 3)

	for _, forbiddenImport := range []string{
		"github.com/dujiao-next/internal/models",
		"github.com/dujiao-next/internal/repository",
		"github.com/dujiao-next/internal/service",
		"gorm.io/gorm",
	} {
		assertProductionImportsAbsent(t, applicationRoot, forbiddenImport)
	}
	for _, forbiddenImport := range []string{
		"github.com/dujiao-next/internal/models",
		"github.com/dujiao-next/internal/repository",
		"github.com/dujiao-next/internal/service",
	} {
		assertProductionImportsAbsent(t, contractRoot, forbiddenImport)
		assertProductionImportsAbsent(t, domainRoot, forbiddenImport)
	}

	assertTypesAbsent(t, filepath.Join(repositoryRoot, "internal", "service"), map[string]struct{}{
		"WalletService":       {},
		"WalletRechargeInput": {},
		"WalletAdjustInput":   {},
		"WalletCreditInput":   {},
	})

	legacyPaths := []string{
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
	}
	for _, relativePath := range legacyPaths {
		absolutePath := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		if _, err := os.Stat(absolutePath); err == nil {
			t.Fatalf("legacy wallet path must stay removed: %s", relativePath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy wallet path %s: %v", relativePath, err)
		}
	}
}

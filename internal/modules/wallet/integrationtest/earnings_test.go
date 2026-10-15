package integrationtest

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/dujiao-next/internal/constants"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type enabledWithdrawal struct{}

func (enabledWithdrawal) GetWalletWithdrawPolicy() (walletcontract.WithdrawPolicy, error) {
	return walletcontract.WithdrawPolicy{Enabled: true, Minimum: cash("20"), Channels: []string{"wechat", "alipay"}, Currency: "CNY"}, nil
}
func cash(s string) money.Amount { return money.FromDecimal(decimal.RequireFromString(s)) }
func earningsWallet(t *testing.T) (*walletapp.Service, *walletstore.Store, *gorm.DB) {
	t.Helper()
	store, db := setupWalletRepositoryTest(t)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	return walletapp.NewService(walletapp.Options{Repository: store, Transactions: store, WithdrawPolicy: enabledWithdrawal{}}), store, db
}
func settle(t *testing.T, store *walletstore.Store, source, amount string) {
	t.Helper()
	err := store.WithinTransaction(func(tx walletcontract.Transaction) error {
		return walletapp.SyncEarningInTransaction(tx, walletcontract.EarningInput{UserID: 1, Reference: source, Currency: "CNY", Amount: cash(amount), Type: "affiliate_earning"})
	})
	if err != nil {
		t.Fatal(err)
	}
}
func accountIs(t *testing.T, svc *walletapp.Service, balance, cashable, frozen, debt string) {
	t.Helper()
	a, err := svc.GetAccount(1)
	if err != nil {
		t.Fatal(err)
	}
	if a.Balance.String() != cash(balance).String() || a.WithdrawableBalance.String() != cash(cashable).String() || a.FrozenBalance.String() != cash(frozen).String() || a.EarningsDebt.String() != cash(debt).String() {
		t.Fatalf("wallet = balance %s cashable %s frozen %s debt %s; want %s/%s/%s/%s", a.Balance.String(), a.WithdrawableBalance.String(), a.FrozenBalance.String(), a.EarningsDebt.String(), balance, cashable, frozen, debt)
	}
}
func applyInput(key, amount string) walletcontract.WithdrawInput {
	return walletcontract.WithdrawInput{UserID: 1, Amount: cash(amount), Channel: "wechat", Account: "test-only-recipient", RealName: "Local Test", RequestKey: key}
}

func TestUnifiedWalletOriginAndRefunds(t *testing.T) {
	svc, store, db := earningsWallet(t)
	_, _, err := svc.Recharge(walletcontract.RechargeInput{UserID: 1, Amount: cash("100"), Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	err = store.WithinTransaction(func(tx walletcontract.Transaction) error {
		_, _, e := svc.CreditInTransaction(tx, walletcontract.CreditInput{UserID: 1, Amount: cash("50"), Type: "gift_card", Reference: "gift:1", Currency: "CNY"})
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApplyWithdraw(applyInput("principal-test-0001", "20")); !errors.Is(err, walletcontract.ErrWithdrawInsufficient) {
		t.Fatalf("principal withdrawn: %v", err)
	}
	settle(t, store, "affiliate:1", "100")
	settle(t, store, "affiliate:1", "100")
	settle(t, store, "reseller:1:CNY", "40")
	accountIs(t, svc, "290", "140", "0", "0")
	pay := func() {
		t.Helper()
		err := store.WithinTransaction(func(tx walletcontract.Transaction) error {
			_, e := svc.ApplyOrderBalance(tx, walletcontract.OrderBalanceInput{OrderID: 99, UserID: 1, TotalAmount: cash("180"), Currency: "CNY", UseBalance: true})
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	pay()
	accountIs(t, svc, "110", "110", "0", "0")
	err = store.WithinTransaction(func(tx walletcontract.Transaction) error {
		_, e := svc.ReleaseOrderBalance(tx, walletcontract.OrderReleaseInput{OrderID: 99, UserID: 1, WalletPaidAmount: cash("180"), Currency: "CNY", TransactionType: constants.WalletTxnTypeOrderRefund}, nil)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	accountIs(t, svc, "290", "140", "0", "0")
	pay()
	accountIs(t, svc, "110", "110", "0", "0")
	// A 200-yuan order used 180 wallet funds, of which only 30 were earnings.
	for i, part := range []struct{ before, after, amount string }{{"0", "60", "60"}, {"60", "180", "120"}} {
		err := store.WithinTransaction(func(tx walletcontract.Transaction) error {
			_, _, e := svc.CreditInTransaction(tx, walletcontract.CreditInput{UserID: 1, Amount: cash(part.amount), Currency: "CNY", Type: constants.WalletTxnTypeAdminRefund, Reference: fmt.Sprintf("refund:%d", i), RefundFunding: &walletcontract.RefundFunding{RootOrderID: 99, RootTotal: cash("200"), RefundedBefore: cash(part.before), RefundedAfter: cash(part.after)}})
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	accountIs(t, svc, "290", "137", "0", "0")
	// Replay the last refund without creating a second classification or credit.
	err = store.WithinTransaction(func(tx walletcontract.Transaction) error {
		_, _, e := svc.CreditInTransaction(tx, walletcontract.CreditInput{UserID: 1, Amount: cash("120"), Reference: "refund:1", Currency: "CNY", RefundFunding: &walletcontract.RefundFunding{RootOrderID: 99, RootTotal: cash("200"), RefundedBefore: cash("60"), RefundedAfter: cash("180")}})
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	accountIs(t, svc, "290", "137", "0", "0")
	assertWalletLedger(t, db)
}

func TestUnifiedWalletManualWithdrawalClawback(t *testing.T) {
	svc, store, db := earningsWallet(t)
	_, _, err := svc.Recharge(walletcontract.RechargeInput{UserID: 1, Amount: cash("100"), Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	settle(t, store, "affiliate:1", "60")
	input := applyInput("unique-withdraw-0001", "20")
	request, err := svc.ApplyWithdraw(input)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := svc.ApplyWithdraw(input)
	if err != nil || duplicate.ID != request.ID {
		t.Fatalf("retry: %v", err)
	}
	changed := input
	changed.Account = "changed-recipient"
	if _, err := svc.ApplyWithdraw(changed); !errors.Is(err, walletcontract.ErrWithdrawConflict) {
		t.Fatal(err)
	}
	accountIs(t, svc, "140", "40", "20", "0")
	if _, err := svc.ReviewWithdraw(walletcontract.WithdrawReviewInput{ID: request.ID, AdminID: 2, Action: "paid", PayoutReference: "simulated-1"}); !errors.Is(err, walletcontract.ErrWithdrawState) {
		t.Fatal(err)
	}
	if _, err := svc.ReviewWithdraw(walletcontract.WithdrawReviewInput{ID: request.ID, AdminID: 2, Action: "approve"}); err != nil {
		t.Fatal(err)
	}
	settle(t, store, "affiliate:1", "0")
	accountIs(t, svc, "100", "0", "20", "20")
	if _, err := svc.ReviewWithdraw(walletcontract.WithdrawReviewInput{ID: request.ID, AdminID: 2, Action: "paid", PayoutReference: "simulated-1"}); !errors.Is(err, walletcontract.ErrWithdrawDebt) {
		t.Fatal(err)
	}
	if _, err := svc.ReviewWithdraw(walletcontract.WithdrawReviewInput{ID: request.ID, AdminID: 2, Action: "reject", Remark: "local test refund"}); err != nil {
		t.Fatal(err)
	}
	accountIs(t, svc, "100", "0", "0", "0")
	settle(t, store, "reseller:1:CNY", "40")
	request, err = svc.ApplyWithdraw(applyInput("unique-withdraw-0002", "40"))
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"approve", "paid", "paid"} {
		if _, err := svc.ReviewWithdraw(walletcontract.WithdrawReviewInput{ID: request.ID, AdminID: 2, Action: action, PayoutReference: "simulation-no-real-payment"}); err != nil {
			t.Fatal(err)
		}
	}
	accountIs(t, svc, "100", "0", "0", "0")
	settle(t, store, "reseller:1:CNY", "0")
	accountIs(t, svc, "100", "0", "0", "40")
	settle(t, store, "affiliate:2", "50")
	accountIs(t, svc, "110", "10", "0", "0")
	assertWalletLedger(t, db)
}

func TestUnifiedWalletConcurrentSettlementsAndWithdrawals(t *testing.T) {
	svc, store, db := earningsWallet(t)
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- store.WithinTransaction(func(tx walletcontract.Transaction) error {
				return walletapp.SyncEarningInTransaction(tx, walletcontract.EarningInput{UserID: 1, Reference: "affiliate:1", Currency: "CNY", Amount: cash("60"), Type: "affiliate_earning"})
			})
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	accountIs(t, svc, "60", "60", "0", "0")
	result := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.ApplyWithdraw(applyInput(fmt.Sprintf("concurrent-request-%d", i), "40"))
			result <- err
		}(i)
	}
	wg.Wait()
	close(result)
	success := 0
	for err := range result {
		if err == nil {
			success++
		} else if !errors.Is(err, walletcontract.ErrWithdrawInsufficient) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("successful competing withdrawals=%d", success)
	}
	accountIs(t, svc, "20", "20", "40", "0")
	var count int64
	db.Model(&walletdomain.EarningSettlement{}).Count(&count)
	if count != 1 {
		t.Fatal(count)
	}
	assertWalletLedger(t, db)
}

func TestUnifiedWalletRollbackAndCurrency(t *testing.T) {
	svc, store, _ := earningsWallet(t)
	sentinel := errors.New("simulate source write failure")
	err := store.WithinTransaction(func(tx walletcontract.Transaction) error {
		if e := walletapp.SyncEarningInTransaction(tx, walletcontract.EarningInput{UserID: 1, Reference: "affiliate:1", Currency: "CNY", Amount: cash("30"), Type: "affiliate_earning"}); e != nil {
			return e
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	accountIs(t, svc, "0", "0", "0", "0")
	err = store.WithinTransaction(func(tx walletcontract.Transaction) error {
		return walletapp.SyncEarningInTransaction(tx, walletcontract.EarningInput{UserID: 1, Reference: "reseller:1:USD", Currency: "USD", Amount: cash("30")})
	})
	if !errors.Is(err, walletcontract.ErrEarningCurrency) {
		t.Fatal(err)
	}
	accountIs(t, svc, "0", "0", "0", "0")
}

func assertWalletLedger(t *testing.T, db *gorm.DB) {
	t.Helper()
	var rows []walletdomain.Transaction
	if err := db.Order("id ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	running := decimal.Zero
	frozen := decimal.Zero
	for _, row := range rows {
		delta := row.Amount.Decimal
		if row.Direction == constants.WalletTxnDirectionOut {
			delta = delta.Neg()
		}
		if !row.BalanceBefore.Decimal.Equal(running) || !row.BalanceAfter.Decimal.Equal(running.Add(delta)) {
			t.Fatalf("unbalanced transaction %d: before=%s amount=%s after=%s", row.ID, row.BalanceBefore.String(), row.Amount.String(), row.BalanceAfter.String())
		}
		running = running.Add(delta)
		frozen = frozen.Add(row.FrozenAmount.Decimal)
	}
	var account walletdomain.Account
	if err := db.Where("user_id = ?", 1).First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if !account.Balance.Decimal.Equal(running) || !account.FrozenBalance.Decimal.Equal(frozen) {
		t.Fatal("ledger does not reconcile with account")
	}
}

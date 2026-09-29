package integrationtest

import (
	"fmt"
	"sync"
	"testing"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletgormstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestWalletTransactionsDoNotRequestASecondConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	dsn := fmt.Sprintf("file:wallet_concurrency_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &walletdomain.Account{}, &walletdomain.Transaction{}); err != nil {
		t.Fatalf("migrate wallet schema: %v", err)
	}
	store := walletgormstore.New(db)
	wallets := walletapp.NewService(walletapp.Options{Repository: store, Transactions: store})

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get raw db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	const workers = 8
	var wait sync.WaitGroup
	errorsCh := make(chan error, workers)
	done := make(chan struct{})
	for userID := 1; userID <= workers; userID++ {
		wait.Add(1)
		go func(id uint) {
			defer wait.Done()
			_, _, rechargeErr := wallets.Recharge(walletcontract.RechargeInput{
				UserID:   id,
				Amount:   money.FromDecimal(decimal.NewFromInt(100)),
				Currency: "CNY",
				Remark:   fmt.Sprintf("concurrency_%d", id),
			})
			if rechargeErr != nil {
				errorsCh <- fmt.Errorf("user %d: %w", id, rechargeErr)
			}
		}(uint(userID))
	}
	go func() {
		wait.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("wallet transactions deadlocked with MaxOpenConns=1")
	}
	close(errorsCh)
	for err := range errorsCh {
		t.Errorf("recharge failed: %v", err)
	}
	for userID := 1; userID <= workers; userID++ {
		account, err := wallets.GetAccount(uint(userID))
		if err != nil {
			t.Fatalf("get account %d: %v", userID, err)
		}
		if account == nil || !account.Balance.Decimal.Equal(decimal.NewFromInt(100)) {
			t.Fatalf("unexpected balance for user %d: %+v", userID, account)
		}
	}
}

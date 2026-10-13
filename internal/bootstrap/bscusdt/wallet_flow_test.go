package bscusdtbootstrap

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	"github.com/dujiao-next/internal/modules/payment/application/bscusdt"
	chaincontract "github.com/dujiao-next/internal/modules/payment/contract/chain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	chainstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/chain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletpresenter "github.com/dujiao-next/internal/modules/wallet/transport/presenter"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestChainWalletRechargeCreditsExactlyOnceAfterConfirmation(t *testing.T) {
	for _, name := range []string{"bsc", "plasma"} {
		t.Run(name, func(t *testing.T) {
			svc, db, registry := setupPaymentService(t)
			sql, _ := db.DB()
			sql.SetMaxOpenConns(1)
			t.Cleanup(func() { sql.Close() })
			if err := db.AutoMigrate(&paymentdomain.ChainInvoice{}, &paymentdomain.ChainCursor{}, &paymentdomain.ChainTransfer{}); err != nil {
				t.Fatal(err)
			}
			network, _ := chaincontract.ResolveNetwork(name)
			chain := &bscTestChain{head: 100, timestamp: time.Now().UTC().Truncate(time.Second)}
			var callback paymentapp.PaymentCallbackInput
			gateway, err := bscusdt.New(chainstore.New(db), chain, bscusdt.Config{Network: name, Recipient: "0x" + strings.Repeat("6", 40), CNYPerUSDT: "6.66"}, func(_ context.Context, invoice paymentdomain.ChainInvoice, transfer paymentdomain.ChainTransfer) error {
				callback = paymentapp.PaymentCallbackInput{PaymentID: invoice.PaymentID, Status: constants.PaymentStatusSuccess, Amount: money.FromDecimal(decimal.RequireFromString(invoice.FiatAmount)), Currency: invoice.Currency, PaidAt: &transfer.PaidAt, ProviderRef: transfer.TxHash}
				_, err := svc.HandleCallback(callback)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			registry.Register(network.Provider, "", gateway)
			channel := paymentdomain.PaymentChannel{Name: network.ChannelName, ProviderType: network.Provider, ChannelType: network.ChannelType, InteractionMode: constants.PaymentInteractionQR, IsActive: true, ConfigJSON: jsonmap.JSON{}, PaymentTypes: jsonslice.Strings{constants.PaymentTypeOrder, constants.PaymentTypeWallet}}
			user := userdomain.User{Email: "wallet-test@example.test", PasswordHash: "unused", Status: "active"}
			if err := db.Create(&channel).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
			channels, err := svc.GetAvailableChannels(paymentapp.AvailablePaymentChannelFilter{User: &user, PaymentType: constants.PaymentTypeWallet})
			if err != nil || len(channels) != 1 {
				t.Fatalf("recharge channel unavailable: %v", err)
			}
			result, err := svc.CreateWalletRechargePayment(paymentapp.CreateWalletRechargePaymentInput{UserID: user.ID, ChannelID: channel.ID, Amount: money.FromDecimal(decimal.NewFromInt(100)), Currency: "CNY", Context: context.Background()})
			if err != nil {
				t.Fatal(err)
			}
			payload := walletpresenter.NewWalletRechargePaymentPayload(result.Recharge, result.Payment, nil)
			if payload.Chain != name || payload.ChainAmount == "" || payload.WalletAddress == "" || payload.ExpiresAt == nil || result.Payment.OrderID != 0 {
				t.Fatal("missing wallet cashier instructions")
			}
			var invoice paymentdomain.ChainInvoice
			if err := db.First(&invoice, "payment_id = ?", result.Payment.ID).Error; err != nil {
				t.Fatal(err)
			}
			if !invoice.ExpiresAt.Equal(*result.Payment.ExpiredAt) {
				t.Fatal("recharge deadline changed")
			}
			var txnCount int64
			db.Model(&walletdomain.Transaction{}).Count(&txnCount)
			if txnCount != 0 {
				t.Fatal("unpaid recharge credited")
			}
			chain.event = &chaincontract.Log{Address: network.Token, Topics: []string{bscusdt.TransferTopic, "0x" + strings.Repeat("0", 24) + strings.Repeat("1", 40), "0x" + strings.Repeat("0", 24) + invoice.Recipient[2:]}, Data: fmt.Sprintf("0x%064x", decimal.RequireFromString(invoice.Units).BigInt()), BlockNumber: "0x65", BlockHash: fmt.Sprintf("0x%064x", 101), TransactionHash: "0x" + strings.Repeat("a", 64), LogIndex: "0x0"}
			chain.head = 114
			if err := gateway.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			db.Model(&walletdomain.Transaction{}).Count(&txnCount)
			if txnCount != 0 {
				t.Fatal("unconfirmed transfer credited")
			}
			chain.head = 120
			for i := 0; i < 2; i++ {
				if err := gateway.RunOnce(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := svc.HandleCallback(callback); err != nil {
				t.Fatal(err)
			}
			var account walletdomain.Account
			if err := db.First(&account, "user_id = ?", user.ID).Error; err != nil {
				t.Fatal(err)
			}
			db.Model(&walletdomain.Transaction{}).Count(&txnCount)
			if !account.Balance.Equal(decimal.NewFromInt(100)) || txnCount != 1 {
				t.Fatalf("balance=%s transactions=%d", account.Balance.String(), txnCount)
			}
			var recharge walletdomain.RechargeOrder
			if err := db.First(&recharge, result.Recharge.ID).Error; err != nil {
				t.Fatal(err)
			}
			if recharge.Status != constants.WalletRechargeStatusSuccess {
				t.Fatal("recharge not successful")
			}
			if _, err := chainstore.New(db).RechargePayableUntil(context.Background(), result.Payment.ID); err == nil {
				t.Fatal("completed recharge can create another invoice")
			}
			unpaid, err := svc.CreateWalletRechargePayment(paymentapp.CreateWalletRechargePaymentInput{UserID: user.ID, ChannelID: channel.ID, Amount: money.FromDecimal(decimal.NewFromInt(20)), Context: context.Background()})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.ExpireWalletRechargePayment(unpaid.Payment.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := chainstore.New(db).RechargePayableUntil(context.Background(), unpaid.Payment.ID); err == nil {
				t.Fatal("expired recharge remains payable")
			}
			db.First(&account, "user_id = ?", user.ID)
			if !account.Balance.Equal(decimal.NewFromInt(100)) {
				t.Fatal("unpaid expiry changed balance")
			}
		})
	}
}

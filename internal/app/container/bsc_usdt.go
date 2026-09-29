package container

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	"github.com/dujiao-next/internal/modules/payment/application/bscusdt"
	"github.com/dujiao-next/internal/modules/payment/domain"
	bscrpc "github.com/dujiao-next/internal/modules/payment/infrastructure/bscusdt"
	chainstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/chain"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

func (c *Container) initBSCUSDT() error {
	if !c.Config.BSCUSDT.Enabled {
		return nil
	}
	endpoint := strings.TrimSpace(os.Getenv("BSC_RPC_URL"))
	if endpoint == "" {
		endpoint = "https://bsc-rpc.publicnode.com"
	}
	rpc, err := bscrpc.NewRPC(endpoint)
	if err != nil {
		return err
	}
	cfg := c.Config.BSCUSDT
	svc, err := bscusdt.New(chainstore.New(gormdb.DB), rpc, bscusdt.Config{Recipient: cfg.Recipient, CNYPerUSDT: cfg.CNYPerUSDT, Confirmations: cfg.Confirmations}, func(ctx context.Context, invoice domain.ChainInvoice, transfer domain.ChainTransfer) error {
		var payment domain.Payment
		if e := gormdb.DB.WithContext(ctx).First(&payment, invoice.PaymentID).Error; e != nil {
			return e
		}
		fiat, e := decimal.NewFromString(invoice.FiatAmount)
		if e != nil || payment.ProviderType != bscusdt.Provider || payment.Currency != invoice.Currency || !payment.Amount.Decimal.Equal(fiat) {
			return errors.New("bsc_payment_snapshot_mismatch")
		}
		if payment.Status == constants.PaymentStatusInitiated {
			return errors.New("bsc_payment_initialization_pending")
		}
		payload := jsonmap.JSON{}
		for k, v := range payment.ProviderPayload {
			payload[k] = v
		}
		payload["chain_tx_hash"] = transfer.TxHash
		payload["chain_log_index"] = transfer.LogIndex
		payload["chain_block"] = transfer.Block
		payload["chain_transfer_key"] = transfer.Key
		_, e = c.PaymentService.HandleCallback(paymentapp.PaymentCallbackInput{PaymentID: payment.ID, ChannelID: payment.ChannelID, Status: constants.PaymentStatusSuccess, ProviderRef: transfer.TxHash, Amount: money.FromDecimal(fiat), Currency: invoice.Currency, PaidAt: &transfer.PaidAt, Payload: payload})
		return e
	})
	if err != nil {
		return err
	}
	c.PaymentProviderRegistry.Register(bscusdt.Provider, "", svc)
	c.BSCUSDTService = svc
	// Explicit enablement creates one ordinary, editable payment channel. It
	// never overwrites merchant channel settings or re-enables a disabled channel.
	var count int64
	if err = gormdb.DB.Model(&domain.PaymentChannel{}).Where("provider_type = ?", bscusdt.Provider).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		managedKey := "bsc-usdt-default"
		channel := domain.PaymentChannel{ManagedKey: &managedKey, Name: "USDT · BNB Smart Chain (BEP-20)", ProviderType: bscusdt.Provider, ChannelType: "usdt-bep20", InteractionMode: constants.PaymentInteractionQR, IsActive: true, ConfigJSON: jsonmap.JSON{}, PaymentTypes: jsonslice.Strings{"order"}, CreatedAt: time.Now()}
		if err = gormdb.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&channel).Error; err != nil {
			return err
		}
	}
	return nil
}

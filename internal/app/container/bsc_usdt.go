package container

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	"github.com/dujiao-next/internal/modules/payment/application/bscusdt"
	chaincontract "github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/domain"
	bscrpc "github.com/dujiao-next/internal/modules/payment/infrastructure/bscusdt"
	chainstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/chain"
	tronrpc "github.com/dujiao-next/internal/modules/payment/infrastructure/tronusdt"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

func (c *Container) initBSCUSDT() error {
	svc, err := c.initChainUSDT(c.Config.BSCUSDT, "bsc")
	c.BSCUSDTService = svc
	return err
}

func (c *Container) initPlasmaUSDT() error {
	svc, err := c.initChainUSDT(c.Config.PlasmaUSDT, "plasma")
	c.PlasmaUSDTService = svc
	return err
}

func (c *Container) initTronUSDT() error {
	svc, err := c.initChainUSDT(c.Config.TronUSDT, "tron")
	c.TronUSDTService = svc
	return err
}

func (c *Container) initChainUSDT(cfg config.BSCUSDTConfig, networkName string) (*bscusdt.Service, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	network, err := chaincontract.ResolveNetwork(networkName)
	if err != nil {
		return nil, err
	}
	prefix := strings.ToUpper(network.Name)
	endpoint := strings.TrimSpace(os.Getenv(prefix + "_RPC_URL"))
	receiptEndpoint := strings.TrimSpace(os.Getenv(prefix + "_RECEIPT_RPC_URL"))
	if endpoint == "" {
		if network.Name == "bsc" {
			endpoint = "https://bsc-rpc.publicnode.com"
			if receiptEndpoint == "" {
				receiptEndpoint = "https://bsc-dataseed.bnbchain.org"
			}
		} else if network.Name == "tron" {
			endpoint = tronrpc.DefaultEndpoint
		} else {
			endpoint = "https://rpc.plasma.to"
		}
	}
	var rpc chaincontract.Reader
	if network.Name == "tron" {
		rpc, err = tronrpc.NewRPC(endpoint, os.Getenv("TRON_API_KEY"))
	} else {
		rpc, err = bscrpc.NewRPCForNetwork(endpoint, receiptEndpoint, network.Name)
	}
	if err != nil {
		return nil, err
	}
	svc, err := bscusdt.New(chainstore.New(gormdb.DB), rpc, bscusdt.Config{Network: network.Name, Recipient: cfg.Recipient, CNYPerUSDT: cfg.CNYPerUSDT, Confirmations: cfg.Confirmations}, func(ctx context.Context, invoice domain.ChainInvoice, transfer domain.ChainTransfer) error {
		var payment domain.Payment
		if e := gormdb.DB.WithContext(ctx).First(&payment, invoice.PaymentID).Error; e != nil {
			return e
		}
		fiat, e := decimal.NewFromString(invoice.FiatAmount)
		if e != nil || payment.ProviderType != network.Provider || payment.ChannelType != network.ChannelType || payment.Currency != invoice.Currency || !payment.Amount.Decimal.Equal(fiat) || invoice.Stream != transfer.Stream {
			return errors.New("chain_payment_snapshot_mismatch")
		}
		if payment.Status == constants.PaymentStatusInitiated {
			return errors.New("chain_payment_initialization_pending")
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
		return nil, err
	}
	c.PaymentProviderRegistry.Register(network.Provider, "", svc)
	// Explicit enablement creates one ordinary, editable payment channel. It
	// never overwrites merchant channel settings or re-enables a disabled channel.
	var count int64
	if err = gormdb.DB.Model(&domain.PaymentChannel{}).Where("provider_type = ?", network.Provider).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		managedKey := network.Name + "-usdt-default"
		channel := domain.PaymentChannel{ManagedKey: &managedKey, Name: network.ChannelName, ProviderType: network.Provider, ChannelType: network.ChannelType, InteractionMode: constants.PaymentInteractionQR, IsActive: true, ConfigJSON: jsonmap.JSON{}, PaymentTypes: jsonslice.Strings{constants.PaymentTypeOrder, constants.PaymentTypeWallet}, CreatedAt: time.Now()}
		if err = gormdb.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&channel).Error; err != nil {
			return nil, err
		}
	}
	return svc, nil
}

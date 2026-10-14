package container

import (
	"path/filepath"
	"testing"

	"github.com/dujiao-next/internal/config"
	chaincontract "github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestChainUSDTStartupCreatesIndependentChannelsAndPreservesMerchantSettings(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "channels.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	previous := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previous; sql.Close() })
	if err = db.AutoMigrate(&domain.PaymentChannel{}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"BSC_RPC_URL", "BSC_RECEIPT_RPC_URL", "PLASMA_RPC_URL", "PLASMA_RECEIPT_RPC_URL"} {
		t.Setenv(key, "")
	}
	c := &Container{Config: &config.Config{}, PaymentProviderRegistry: newPaymentProviderRegistry()}
	if err = c.initPlasmaUSDT(); err != nil || c.PlasmaUSDTService != nil {
		t.Fatal("disabled Plasma listener initialized", err)
	}
	settings := config.BSCUSDTConfig{Enabled: true, Recipient: "0x66a8bab15067eab1efc948ac03684f0397c8416f", CNYPerUSDT: "6.66", Confirmations: 15}
	c.Config.BSCUSDT, c.Config.PlasmaUSDT = settings, settings
	if err = c.initBSCUSDT(); err != nil {
		t.Fatal(err)
	}
	if err = c.initPlasmaUSDT(); err != nil {
		t.Fatal(err)
	}
	var channels []domain.PaymentChannel
	if err = db.Order("id").Find(&channels).Error; err != nil || len(channels) != 2 {
		t.Fatal("missing independent payment channels", err)
	}
	if channels[0].ProviderType != "bscusdt" || channels[0].ChannelType != "usdt-bep20" || channels[1].ProviderType != "plasmausdt" || channels[1].ChannelType != "usdt-plasma" {
		t.Fatal("incorrect network channel registration")
	}
	for _, channel := range channels {
		if len(channel.PaymentTypes) != 2 || channel.PaymentTypes[0] != "order" || channel.PaymentTypes[1] != "wallet" {
			t.Fatal("new chain channel must support both orders and wallet recharge")
		}
	}
	if gateway, ok := c.PaymentProviderRegistry.Lookup("plasmausdt", "usdt-plasma"); !ok || gateway != c.PlasmaUSDTService {
		t.Fatal("Plasma payment gateway not registered")
	}
	if err = db.Model(&channels[1]).Updates(map[string]any{"is_active": false, "name": "Merchant Plasma name"}).Error; err != nil {
		t.Fatal(err)
	}
	if err = c.initPlasmaUSDT(); err != nil {
		t.Fatal(err)
	}
	var after []domain.PaymentChannel
	if err = db.Order("id").Find(&after).Error; err != nil || len(after) != 2 || after[1].IsActive || after[1].Name != "Merchant Plasma name" || after[0].ProviderType != "bscusdt" {
		t.Fatal("restart overwrote merchant channel or created duplicates", err)
	}
}

func TestTronChannelSupportsOrdersAndWalletWithoutReplacingMerchantSettings(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "tron.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	previous := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previous; sql.Close() })
	if err = db.AutoMigrate(&domain.PaymentChannel{}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRON_RPC_URL", "")
	t.Setenv("TRON_API_KEY", "")
	c := &Container{Config: &config.Config{}, PaymentProviderRegistry: newPaymentProviderRegistry()}
	if err = c.initTronUSDT(); err != nil || c.TronUSDTService != nil {
		t.Fatal("disabled listener initialized", err)
	}
	network, _ := chaincontract.ResolveNetwork("tron")
	c.Config.TronUSDT = config.BSCUSDTConfig{Enabled: true, Recipient: network.DisplayAddress("0x66a8bab15067eab1efc948ac03684f0397c8416f"), CNYPerUSDT: "6.66", Confirmations: 15}
	if err = c.initTronUSDT(); err != nil {
		t.Fatal(err)
	}
	var channel domain.PaymentChannel
	if err = db.First(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if channel.ProviderType != "tronusdt" || channel.ChannelType != "usdt-trc20" || len(channel.PaymentTypes) != 2 || channel.PaymentTypes[0] != "order" || channel.PaymentTypes[1] != "wallet" {
		t.Fatal("incorrect TRON channel")
	}
	if gateway, ok := c.PaymentProviderRegistry.Lookup("tronusdt", "usdt-trc20"); !ok || gateway != c.TronUSDTService {
		t.Fatal("missing native gateway")
	}
	if err = db.Model(&channel).Updates(map[string]any{"is_active": false, "name": "Merchant TRON"}).Error; err != nil {
		t.Fatal(err)
	}
	if err = c.initTronUSDT(); err != nil {
		t.Fatal(err)
	}
	var channels []domain.PaymentChannel
	if err = db.Find(&channels).Error; err != nil || len(channels) != 1 || channels[0].IsActive || channels[0].Name != "Merchant TRON" {
		t.Fatal("restart overwrote channel", err)
	}
}

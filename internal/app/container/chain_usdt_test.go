package container

import (
	"path/filepath"
	"testing"

	"github.com/dujiao-next/internal/config"
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

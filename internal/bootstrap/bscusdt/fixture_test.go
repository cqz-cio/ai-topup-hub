package bscusdtbootstrap

import (
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/provider"
	paymentstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func setupPaymentService(t *testing.T) (*paymentapp.PaymentService, *gorm.DB, *provider.Registry) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "payment.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&userdomain.User{}, &orderdomain.Order{}, &orderdomain.OrderItem{}, &fulfillmentdomain.Fulfillment{}, &productdomain.Product{}, &productdomain.ProductSKU{}, &paymentdomain.PaymentChannel{}, &paymentdomain.Payment{}); err != nil {
		t.Fatal(err)
	}
	registry := provider.NewRegistry()
	svc := paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{OrderStore: orderstore.New(db, "test-guest-key"), PaymentStore: paymentstore.New(db, "test-guest-key"), ChannelStore: paymentstore.NewChannelStore(db), ProductRepo: productstore.NewProductStore(db), ProductSKURepo: productstore.NewSKUStore(db), UserStore: userstore.New(db), PaymentProviderRegistry: registry, ExpireMinutes: 15})
	return svc, db, registry
}

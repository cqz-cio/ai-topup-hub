package autorechargebootstrap

import (
	"context"
	"github.com/dujiao-next/internal/constants"
	rechargedomain "github.com/dujiao-next/internal/modules/autorecharge/domain"
	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	fulfillmentstore "github.com/dujiao-next/internal/modules/fulfillment/infrastructure/gormstore"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
	"time"
)

func TestPaidOrderAdapterCompletesRealOrderIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "order.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	t.Cleanup(func() { _ = sql.Close() })
	if err = db.AutoMigrate(&orderdomain.Order{}, &orderdomain.OrderItem{}, &fulfillmentdomain.Fulfillment{}, &rechargedomain.Task{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	o := orderdomain.Order{OrderNo: "LOCAL-1", UserID: 7, Status: constants.OrderStatusFulfilling, Currency: "USD", PaidAt: &now, Items: []orderdomain.OrderItem{{SKUID: 9, Quantity: 1, FulfillmentType: constants.FulfillmentTypeManual, TitleJSON: jsonmap.JSON{"en-US": "Plus"}}}}
	if err = db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	store := orderstore.New(db, "test-secret")
	gateway := &Orders{DB: db, Store: store, Fulfillment: fulfillmentapp.New(fulfillmentapp.Options{OrderStore: store, FulfillmentStore: fulfillmentstore.New(db)})}
	ids, err := gateway.Unscheduled(context.Background(), []uint{9}, 10)
	if err != nil || len(ids) != 1 || ids[0] != o.ID {
		t.Fatalf("recovery scan: %v %v", ids, err)
	}
	id, err := gateway.Resolve(context.Background(), o.OrderNo, 7)
	if err != nil || id != o.ID {
		t.Fatal("order number resolution", err)
	}
	if _, err = gateway.Resolve(context.Background(), o.OrderNo, 8); err == nil {
		t.Fatal("cross-user resolution")
	}
	for i := 0; i < 2; i++ {
		if err = gateway.DeliverCode(context.Background(), o.ID, "K7M2P-R8W4X-6NQ9T-H3V5C-Y2D8F"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.GetByID(o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != constants.OrderStatusDelivered || got.Fulfillment == nil || got.Fulfillment.DeliveredBy != nil {
		t.Fatal("delivery failed or invented admin identity")
	}
	var count int64
	db.Model(&fulfillmentdomain.Fulfillment{}).Count(&count)
	if count != 1 {
		t.Fatal("duplicate delivery")
	}
	if err = gateway.DeliverCode(context.Background(), o.ID, "different-code"); err == nil {
		t.Fatal("accepted different task for delivered order")
	}
	gateway.RechargeSKUs = map[uint]bool{9: true}
	mixed := orderdomain.Order{OrderNo: "MIXED", UserID: 7, Status: constants.OrderStatusPaid, Currency: "USD", PaidAt: &now, Items: []orderdomain.OrderItem{{SKUID: 8, Quantity: 1, FulfillmentType: constants.FulfillmentTypeManual}, {SKUID: 9, Quantity: 1, FulfillmentType: constants.FulfillmentTypeManual}}}
	for i := range mixed.Items {
		mixed.Items[i].TitleJSON = jsonmap.JSON{"en-US": "Test item"}
	}
	if err = db.Create(&mixed).Error; err != nil {
		t.Fatal(err)
	}
	view, err := gateway.Get(context.Background(), mixed.ID)
	if err != nil || view == nil || view.SKUID != 9 || view.Eligible {
		t.Fatalf("mixed order must retain mapped SKU for manual review: %+v %v", view, err)
	}
	if err = db.Model(&mixed).Update("deleted_at", now).Error; err != nil {
		t.Fatal(err)
	}
	view, err = gateway.Get(context.Background(), mixed.ID)
	if err != nil || (view != nil && view.Eligible) {
		t.Fatal("deleted order must not be fulfilled", err)
	}
}

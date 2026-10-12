package procurement_test

import (
	"context"
	"sync/atomic"

	"fmt"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/upstream"
	"testing"
	"time"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	"github.com/dujiao-next/internal/config"
	mappinggormstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	procurementapp "github.com/dujiao-next/internal/modules/procurement/application"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	procurementgormstore "github.com/dujiao-next/internal/modules/procurement/infrastructure/gormstore"
	procurementmapping "github.com/dujiao-next/internal/modules/procurement/infrastructure/mappingreader"
	procurementorder "github.com/dujiao-next/internal/modules/procurement/infrastructure/orderreader"
	procurementupstream "github.com/dujiao-next/internal/modules/procurement/infrastructure/upstreamgateway"
	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
	siteconnectiongormstore "github.com/dujiao-next/internal/modules/siteconnection/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ListFilter = procurementcontract.ListFilter
type ProcurementOrder = procurementdomain.Order

var (
	ErrExists = procurementcontract.ErrExists
)

func newTestSiteConnectionService(db *gorm.DB, secretKey, uploadsDir string) *siteconnectionapp.Service {
	return siteconnectionapp.NewService(siteconnectiongormstore.New(db), secretKey, uploadsDir)
}

func setupProcurementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:procurement_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&orderdomain.OrderRefundRecord{},
		&fulfillmentdomain.Fulfillment{},
		&procurementdomain.Order{},
		&siteconnectiondomain.Connection{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	gormdb.DB = db
	return db
}

func createProcTestOrder(t *testing.T, db *gorm.DB, orderNo, status, fulfillmentType string) *orderdomain.Order {
	t.Helper()
	order := &orderdomain.Order{
		OrderNo:        orderNo,
		UserID:         1,
		Status:         status,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	item := &orderdomain.OrderItem{
		OrderID:         order.ID,
		ProductID:       1,
		SKUID:           1,
		Quantity:        1,
		FulfillmentType: fulfillmentType,
		TitleJSON:       jsonmap.JSON{"zh-CN": "Test Product"},
		UnitPrice:       money.FromDecimal(decimal.NewFromInt(100)),
		TotalPrice:      money.FromDecimal(decimal.NewFromInt(100)),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}
	var loaded orderdomain.Order
	if err := db.Preload("Items").First(&loaded, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	return &loaded
}

func createTestProcurementOrder(t *testing.T, db *gorm.DB, connID, localOrderID uint, localOrderNo, status string) *ProcurementOrder {
	t.Helper()
	order := &ProcurementOrder{
		ConnectionID:    connID,
		LocalOrderID:    localOrderID,
		LocalOrderNo:    localOrderNo,
		Status:          status,
		LocalSellAmount: money.FromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		TraceID:         "test-trace-id",
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create procurement order failed: %v", err)
	}
	return order
}

func newTestProcurementService(db *gorm.DB, connections *siteconnectionapp.Service) *procurementapp.Service {
	options := procurementOptionsForTest(db)
	options.Connections = procurementupstream.New(connections)
	return procurementapp.NewService(options)
}

func procurementOptionsForTest(db *gorm.DB) procurementapp.Options {
	orders := ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	return procurementapp.Options{
		Repository:      procurementgormstore.New(db),
		Orders:          procurementorder.New(orders),
		ProductMappings: procurementmapping.NewProducts(mappinggormstore.NewMappingStore(db)),
		SKUMappings:     procurementmapping.NewSKUs(mappinggormstore.NewSKUMappingStore(db)),
		OrderLifecycle:  procurementgormstore.NewLifecycle(db, nil, nil, config.EmailConfig{}),
	}
}

// The wire protocol is covered in internal/upstream/acg_faka_test.go. This
// fixture exercises the real DB claim, gateway, order lifecycle and card record.
type acgProcurementAdapter struct {
	upstream.Adapter
	calls     atomic.Int32
	result    *upstream.CreateUpstreamOrderResp
	err       error
	reference string
	request   upstream.CreateUpstreamOrderReq
}

func (*acgProcurementAdapter) RequiresSingleSubmission() bool { return true }
func (a *acgProcurementAdapter) CreateOrder(_ context.Context, request upstream.CreateUpstreamOrderReq) (*upstream.CreateUpstreamOrderResp, error) {
	a.calls.Add(1)
	a.request = request
	return a.result, a.err
}
func (a *acgProcurementAdapter) GetOrderByReference(_ context.Context, no string) (*upstream.UpstreamOrderDetail, error) {
	a.reference = no
	return &upstream.UpstreamOrderDetail{OrderNo: no, Status: "delivered", Fulfillment: &upstream.UpstreamFulfillment{Type: "auto", Status: "delivered", Payload: "DELAYED-CARD"}}, nil
}

type acgProcurementProvider struct{ adapter *acgProcurementAdapter }

func (p *acgProcurementProvider) GetByID(id uint) (*siteconnectiondomain.Connection, error) {
	return &siteconnectiondomain.Connection{ID: id, Protocol: constants.ConnectionProtocolACGFaka, RetryMax: 2, RetryIntervals: "[30,60]"}, nil
}
func (p *acgProcurementProvider) GetAdapter(*siteconnectiondomain.Connection) (upstream.Adapter, error) {
	return p.adapter, nil
}

func acgProcurementFixture(t *testing.T, status string, adapter *acgProcurementAdapter) (*gorm.DB, *procurementapp.Service, *ProcurementOrder, *orderdomain.Order) {
	t.Helper()
	db := setupProcurementTestDB(t)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	order := createProcTestOrder(t, db, "ACG-ORDER-1", status, constants.FulfillmentTypeUpstream)
	if err := db.Model(&orderdomain.OrderItem{}).Where("order_id = ?", order.ID).Update("cost_price", "33.00").Error; err != nil {
		t.Fatal(err)
	}
	pm := &mappingdomain.Mapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 4, IsActive: true}
	if err := db.Create(pm).Error; err != nil {
		t.Fatal(err)
	}
	sm := &mappingdomain.SKUMapping{ProductMappingID: pm.ID, LocalSKUID: 1, UpstreamSKUID: 4, UpstreamIsActive: true}
	if err := db.Create(sm).Error; err != nil {
		t.Fatal(err)
	}
	// Construct the same production service, replacing only the supplier wire.
	options := procurementOptionsForTest(db)
	options.Connections = procurementupstream.New(&acgProcurementProvider{adapter: adapter})
	svc := procurementapp.NewService(options)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, "pending")
	return db, svc, proc, order
}

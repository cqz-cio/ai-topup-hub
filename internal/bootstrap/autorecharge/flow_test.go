package autorechargebootstrap

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	rechargeapp "github.com/dujiao-next/internal/modules/autorecharge/application"
	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	"github.com/dujiao-next/internal/modules/autorecharge/domain"
	rechargestore "github.com/dujiao-next/internal/modules/autorecharge/infrastructure/gormstore"
	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	fulfillmentstore "github.com/dujiao-next/internal/modules/fulfillment/infrastructure/gormstore"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type flowPartner struct{ pays int }

func (*flowPartner) Ready() bool                                   { return true }
func (*flowPartner) Check(context.Context, contract.Binding) error { return nil }
func (*flowPartner) InspectAccount(context.Context, string) (*contract.Account, error) {
	return &contract.Account{ID: "verified-account", Plan: "free"}, nil
}
func (p *flowPartner) Pay(context.Context, string, contract.PayRequest) (*contract.Result, error) {
	p.pays++
	return &contract.Result{OperationID: "op-flow", State: "queued"}, nil
}
func (*flowPartner) Poll(context.Context, string) (*contract.Result, error) {
	yes := true
	return &contract.Result{OperationID: "op-flow", State: "succeeded", SubscriptionSynced: &yes}, nil
}
func (*flowPartner) Finalize(context.Context, contract.FinalizeRequest) (*contract.Result, error) {
	return nil, contract.ErrInvalid
}
func (*flowPartner) SubscriptionActive(context.Context, string, string) (bool, error) {
	return true, nil
}

type flowCards struct{}

func (flowCards) Reserve(context.Context, string, string) (string, error) {
	return "test-card-ref", nil
}
func (flowCards) Credentials(context.Context, string) (contract.Card, error) {
	return contract.Card{Number: "4242424242424242", ExpMonth: "12", ExpYear: "2030", CVC: "123"}, nil
}

func TestRealPaidOrderDeliveryRedemptionAndCrashRecovery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "flow.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sql.Close() })
	if err = db.AutoMigrate(&orderdomain.Order{}, &orderdomain.OrderItem{}, &fulfillmentdomain.Fulfillment{}, &domain.Task{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	o := orderdomain.Order{OrderNo: "REDEEM-FLOW", Status: constants.OrderStatusPaid, Currency: "USD", PaidAt: &now, GuestEmail: "guest@example.invalid", Items: []orderdomain.OrderItem{{SKUID: 9, Quantity: 1, FulfillmentType: constants.FulfillmentTypeManual, TitleJSON: jsonmap.JSON{"en-US": "Plus"}}}}
	if err = db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	os := orderstore.New(db, "test-secret")
	gateway := &Orders{DB: db, Store: os, Fulfillment: fulfillmentapp.New(fulfillmentapp.Options{OrderStore: os, FulfillmentStore: fulfillmentstore.New(db)}), RechargeSKUs: map[uint]bool{9: true}, RedeemURL: "https://redeem.example.invalid"}
	rs := rechargestore.New(db)
	partner := &flowPartner{}
	svc, err := rechargeapp.New(rechargeapp.Options{Store: rs, Orders: gateway, Partner: partner, Cards: flowCards{}, Secret: strings.Repeat("k", 32), Bindings: []contract.Binding{{SKUID: 9, Plan: "chatgptplusplan", Region: "US", RegionVersion: 12, Channel: "3", CardRule: "test"}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// Recover a missed paid callback and issue the entitlement for a guest order.
	if err = svc.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := os.GetByID(o.ID)
	if err != nil || got.Fulfillment == nil || got.Status != constants.OrderStatusDelivered || partner.pays != 0 {
		t.Fatal("paid order did not deliver code independently")
	}
	payload := got.Fulfillment.Payload
	code := strings.Split(payload, "\n")[1]
	if _, err = domain.NormalizeRedemptionCode(code); err != nil {
		t.Fatal("invalid delivered code")
	}
	// Simulate crash after delivery committed but before issued state was saved.
	if err = db.Model(&domain.Task{}).Where("order_id = ?", o.ID).Updates(map[string]interface{}{"state": "issuing", "issued_at": nil}).Error; err != nil {
		t.Fatal(err)
	}
	if err = svc.Advance(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&fulfillmentdomain.Fulfillment{}).Count(&count)
	if count != 1 {
		t.Fatal("crash recovery duplicated code delivery")
	}
	v, err := svc.CheckAccount(ctx, code, `{"accessToken":"test","sessionToken":"test"}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ConfirmCode(ctx, code, v.ConfirmationToken); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err = svc.Advance(ctx, o.ID); err != nil {
			t.Fatal(err)
		}
	}
	task, err := rs.Get(ctx, o.ID)
	if err != nil || task.State != "completed" || task.RedeemedAt == nil || partner.pays != 1 {
		t.Fatal("redemption did not complete exactly once")
	}
	got, err = os.GetByID(o.ID)
	if err != nil || got.Fulfillment.Payload != payload {
		t.Fatal("redemption replaced original card delivery")
	}
	if task.SessionCipher != "" || task.AccountCipher != "" || task.ActiveAccountHash != nil {
		t.Fatal("terminal secrets or account lock retained")
	}
}

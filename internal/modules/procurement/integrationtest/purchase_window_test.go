package procurement_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	procurementapp "github.com/dujiao-next/internal/modules/procurement/application"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	procurementgormstore "github.com/dujiao-next/internal/modules/procurement/infrastructure/gormstore"
	procurementupstream "github.com/dujiao-next/internal/modules/procurement/infrastructure/upstreamgateway"
	"github.com/dujiao-next/internal/upstream"
)

// A nil supplier result deliberately panics if a forbidden submission reaches
// the fake wire. No production configuration or network is used in these tests.
func TestPurchaseWindowPreservesHistoricalTasks(t *testing.T) {
	for _, status := range []string{"pending", "failed", "rejected", "submitting", "manual_review", "fulfilled"} {
		t.Run(status, func(t *testing.T) {
			adapter := &acgProcurementAdapter{}
			db, _, proc, order := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
			cutoff := time.Now().Add(-10 * time.Minute)
			// A late historical payment confirmation must not pass the ID fence.
			if err := db.Model(order).Update("paid_at", cutoff.Add(time.Minute)).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(proc).Updates(map[string]any{"status": status, "created_at": cutoff.Add(-time.Minute), "updated_at": cutoff.Add(-time.Minute), "error_message": "preserve historical evidence", "retry_count": 2}).Error; err != nil {
				t.Fatal(err)
			}
			var before, after procurementdomain.Order
			if err := db.First(&before, proc.ID).Error; err != nil {
				t.Fatal(err)
			}
			q := &workflowQueue{}
			options := procurementOptionsForTest(db)
			options.Queue = q
			options.Connections = procurementupstream.New(&acgProcurementProvider{adapter: adapter})
			options.PurchaseWindow = &procurementdomain.PurchaseWindow{OrderIDFloor: order.ID, PaidAfter: cutoff}
			svc := procurementapp.NewService(options)
			for i := 0; i < 2; i++ {
				if err := svc.CreateForOrder(order.ID); err != nil {
					t.Fatal(err)
				}
				if err := svc.SubmitToUpstream(proc.ID); err != nil {
					t.Fatal(err)
				}
				if err := svc.RecoverPaidOrders(); err != nil {
					t.Fatal(err)
				}
			}
			if status == "failed" || status == "rejected" {
				if err := svc.RetryManual(proc.ID); !errors.Is(err, procurementcontract.ErrOutsidePurchaseWindow) {
					t.Fatalf("historical manual retry: %v", err)
				}
			}
			if err := db.First(&after, proc.ID).Error; err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(before)
			a, _ := json.Marshal(after)
			if string(a) != string(b) || len(q.submits) != 0 || len(q.polls) != 0 || adapter.calls.Load() != 0 {
				t.Fatal("historical task changed or reached purchase queue")
			}
		})
	}
}

func TestPurchaseWindowRejectsIneligibleOrdersBeforeIntent(t *testing.T) {
	for _, scenario := range []string{"old id with late payment", "paid before cutoff", "paid at cutoff", "missing paid timestamp", "unpaid", "refunded", "historical parent", "unpaid parent", "zero cutoff"} {
		t.Run(scenario, func(t *testing.T) {
			adapter := &acgProcurementAdapter{}
			db, _, proc, order := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
			if err := db.Delete(proc).Error; err != nil {
				t.Fatal(err)
			}
			cutoff := time.Now().Add(-10 * time.Minute)
			window := &procurementdomain.PurchaseWindow{PaidAfter: cutoff}
			fields := map[string]any{"paid_at": cutoff.Add(time.Minute)}
			switch scenario {
			case "old id with late payment":
				window.OrderIDFloor = order.ID
			case "paid before cutoff":
				fields["paid_at"] = cutoff.Add(-time.Second)
			case "paid at cutoff":
				fields["paid_at"] = cutoff
			case "missing paid timestamp":
				fields["paid_at"] = nil
			case "unpaid":
				fields["status"] = constants.OrderStatusPendingPayment
			case "refunded":
				fields["refunded_amount"] = "1.00"
			case "zero cutoff":
				window.PaidAfter = time.Time{}
			case "historical parent", "unpaid parent":
				parent := createProcTestOrder(t, db, "PARENT", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
				fields["parent_id"] = parent.ID
				if scenario == "unpaid parent" {
					if err := db.Model(parent).Updates(map[string]any{"status": constants.OrderStatusPendingPayment, "paid_at": cutoff.Add(time.Minute)}).Error; err != nil {
						t.Fatal(err)
					}
				} else {
					if err := db.Model(parent).Update("paid_at", cutoff.Add(-time.Minute)).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := db.Model(order).Updates(fields).Error; err != nil {
				t.Fatal(err)
			}
			q := &workflowQueue{}
			options := procurementOptionsForTest(db)
			options.Queue = q
			options.PurchaseWindow = window
			svc := procurementapp.NewService(options)
			if err := svc.CreateForOrder(order.ID); err != nil {
				t.Fatal(err)
			}
			if err := svc.RecoverPaidOrders(); err != nil {
				t.Fatal(err)
			}
			var count int64
			if err := db.Model(&procurementdomain.Order{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 || len(q.submits) != 0 {
				t.Fatal("ineligible order created a purchase intent")
			}
		})
	}
}

func TestPurchaseWindowRejectsOldIntentForOtherwiseNewPayment(t *testing.T) {
	adapter := &acgProcurementAdapter{}
	db, _, proc, order := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	cutoff := time.Now().Add(-10 * time.Minute)
	if err := db.Model(order).Update("paid_at", cutoff.Add(time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(proc).Update("created_at", cutoff).Error; err != nil {
		t.Fatal(err)
	}
	options := procurementOptionsForTest(db)
	options.PurchaseWindow = &procurementdomain.PurchaseWindow{PaidAfter: cutoff}
	options.Connections = procurementupstream.New(&acgProcurementProvider{adapter: adapter})
	if err := procurementapp.NewService(options).SubmitToUpstream(proc.ID); err != nil {
		t.Fatal(err)
	}
	if adapter.calls.Load() != 0 {
		t.Fatal("old intent was submitted")
	}
}

func TestPurchaseWindowRecoveryFiltersBeforeLimitAcrossTimezones(t *testing.T) {
	db := setupProcurementTestDB(t)
	cutoff := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	var floor uint
	for i := 0; i < 105; i++ {
		createProcTestOrder(t, db, fmt.Sprintf("OLD-MISSING-%d", i), constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
		old := createProcTestOrder(t, db, fmt.Sprintf("OLD-%d", i), constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
		floor = old.ID
		p := createTestProcurementOrder(t, db, 1, old.ID, old.OrderNo, "failed")
		if err := db.Model(p).Updates(map[string]any{"created_at": cutoff.Add(-time.Hour), "updated_at": cutoff.Add(-time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	window := &procurementdomain.PurchaseWindow{OrderIDFloor: floor, PaidAfter: cutoff}
	for _, offset := range []int{8, -7} {
		zone := time.FixedZone("stored", offset*60*60)
		for _, delta := range []time.Duration{-time.Second, time.Second} {
			o := createProcTestOrder(t, db, fmt.Sprintf("ZONE-%d-%d", offset, delta), constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
			if err := db.Model(o).Update("paid_at", cutoff.Add(delta).In(zone)).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	store := procurementgormstore.New(db)
	ids, err := store.FindPaidOrdersMissingProcurement(2, window)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != floor+2 || ids[1] != floor+4 {
		t.Fatalf("wrong instant comparison: %v", ids)
	}
	for _, id := range ids {
		p := createTestProcurementOrder(t, db, 1, id, fmt.Sprintf("ZONE-PROC-%d", id), "failed")
		if err := db.Model(p).Updates(map[string]any{"created_at": cutoff.Add(time.Minute), "updated_at": cutoff.Add(time.Minute)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	q := &workflowQueue{}
	options := procurementOptionsForTest(db)
	options.Queue = q
	options.PurchaseWindow = window
	if err := procurementapp.NewService(options).RecoverPaidOrders(); err != nil {
		t.Fatal(err)
	}
	if len(q.submits) != 2 || q.submits[0] <= 105 || q.submits[1] <= 105 {
		t.Fatalf("historical backlog starved new work: %v", q.submits)
	}
}

func TestPurchaseWindowNewSplitOrderContinuesAfterFirstDelivery(t *testing.T) {
	adapter := &acgProcurementAdapter{result: &upstream.CreateUpstreamOrderResp{OK: true, OrderNo: "FIXTURE", Amount: "33.00", Currency: "CNY", Fulfillment: &upstream.UpstreamFulfillment{Type: "auto", Status: "delivered", Payload: "SIMULATED-CARD"}}}
	db, _, proc, first := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	if err := db.Delete(proc).Error; err != nil {
		t.Fatal(err)
	}
	parent := createProcTestOrder(t, db, "NEW-PARENT", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	second := createProcTestOrder(t, db, "NEW-CHILD-2", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	paid := time.Now().Add(-time.Minute)
	if err := db.Model(&orderdomain.Order{}).Where("id IN ?", []uint{parent.ID, first.ID, second.ID}).Update("paid_at", paid).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&orderdomain.Order{}).Where("id IN ?", []uint{first.ID, second.ID}).Update("parent_id", parent.ID).Error; err != nil {
		t.Fatal(err)
	}
	q := &workflowQueue{}
	options := procurementOptionsForTest(db)
	options.Queue = q
	options.PurchaseWindow = &procurementdomain.PurchaseWindow{PaidAfter: paid.Add(-time.Minute)}
	options.Connections = procurementupstream.New(&acgProcurementProvider{adapter: adapter})
	svc := procurementapp.NewService(options)
	if err := svc.CreateForOrder(parent.ID); err != nil {
		t.Fatal(err)
	}
	if len(q.submits) != 2 {
		t.Fatalf("expected two child purchase intents: %v", q.submits)
	}
	for _, id := range q.submits {
		if err := svc.SubmitToUpstream(id); err != nil {
			t.Fatal(err)
		}
		if err := svc.SubmitToUpstream(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.First(parent, parent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if parent.Status != constants.OrderStatusDelivered || adapter.calls.Load() != 2 {
		t.Fatal("remaining new child was blocked or purchased twice")
	}
}

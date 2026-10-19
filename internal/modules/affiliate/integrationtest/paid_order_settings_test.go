package integrationtest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	affiliateapp "github.com/dujiao-next/internal/modules/affiliate/application"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
)

func TestPaidOrderWithSingleConnectionAndDatabaseSettings(t *testing.T) {
	for _, tc := range []struct {
		name        string
		enabled     bool
		rate        float64
		confirmDays int
		want        int64
	}{
		{"disabled", false, 2, 30, 0},
		{"zero_rate", true, 0, 30, 0},
		{"pending_commission", true, 2, 30, 1},
		{"immediately_available", true, 2, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _, store, order := affiliateWalletFixture(t)
			if err := db.AutoMigrate(&settingsstore.SettingRecord{}); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(order).Error; err != nil {
				t.Fatal(err)
			}
			settings := settingsapp.NewService(settingsstore.New(db))
			if _, err := settings.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{Enabled: tc.enabled, CommissionRate: tc.rate, ConfirmDays: tc.confirmDays}); err != nil {
				t.Fatal(err)
			}
			// Bound the settings read so the original nested-connection deadlock
			// fails this test instead of hanging the suite. Both stores share the
			// actual production-sized SQL pool; no in-memory settings substitute.
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			svc := affiliateapp.NewService(store, nil, nil, nil, settingsapp.NewService(settingsstore.New(db.WithContext(ctx))))
			raw, _ := db.DB()
			before := raw.Stats()
			if err := svc.HandleOrderPaid(order.ID); err != nil {
				t.Fatalf("paid hook blocked or failed: %v", err)
			}
			if raw.Stats().WaitCount != before.WaitCount {
				t.Fatal("single paid hook waited for its own database connection")
			}
			// Redelivered callbacks must retain the transaction's idempotency.
			var wg sync.WaitGroup
			errs := make(chan error, 4)
			for range 4 {
				wg.Add(1)
				go func() { defer wg.Done(); errs <- svc.HandleOrderPaid(order.ID) }()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := db.First(order, order.ID).Error; err != nil {
				t.Fatal(err)
			}
			if !order.AffiliateCommissionProcessed {
				t.Fatal("paid hook was not durably marked complete")
			}
			var count int64
			if err := db.Model(&affiliatedomain.Commission{}).Where("order_id = ?", order.ID).Count(&count).Error; err != nil || count != tc.want {
				t.Fatalf("commissions=%d want=%d err=%v", count, tc.want, err)
			}
			if raw.Stats().InUse != 0 {
				t.Fatal("paid hook retained a connection")
			}
		})
	}
}

func TestPaidOrderSettingsFailureLeavesRecoveryMarkerUnset(t *testing.T) {
	db, _, store, order := affiliateWalletFixture(t)
	if err := db.AutoMigrate(&settingsstore.SettingRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := affiliateapp.NewService(store, nil, nil, nil, settingsapp.NewService(settingsstore.New(db.WithContext(ctx))))
	if err := svc.HandleOrderPaid(order.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected setting read error, got %v", err)
	}
	if err := db.First(order, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if order.AffiliateCommissionProcessed {
		t.Fatal("failed hook incorrectly marked complete")
	}
	// Recovery uses a fresh settings read and can finish on the same pool.
	svc = affiliateapp.NewService(store, nil, nil, nil, settingsapp.NewService(settingsstore.New(db)))
	if err := svc.HandleOrderPaid(order.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.First(order, order.ID).Error; err != nil || !order.AffiliateCommissionProcessed {
		t.Fatalf("recovery failed: %v", err)
	}
}

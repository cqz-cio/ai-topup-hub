package gormstore_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	"github.com/dujiao-next/internal/modules/coupon/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newCouponStoreTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:coupon_store_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&coupondomain.Coupon{}, &coupondomain.CouponUsage{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func createCouponStoreFixture(t *testing.T, store *gormstore.Store) *coupondomain.Coupon {
	t.Helper()

	coupon := &coupondomain.Coupon{
		Code:        "DELETE_ME",
		Type:        constants.CouponTypeFixed,
		Value:       money.FromDecimal(decimal.NewFromInt(10)),
		MinAmount:   money.FromDecimal(decimal.Zero),
		MaxDiscount: money.FromDecimal(decimal.Zero),
		UsedCount:   2,
		ScopeType:   constants.ScopeTypeProduct,
		ScopeRefIDs: "[1]",
		IsActive:    true,
	}
	if err := store.Create(coupon); err != nil {
		t.Fatalf("create coupon: %v", err)
	}
	return coupon
}

func TestCouponStoreSoftDeleteHidesCouponFromEveryReadAndWritePath(t *testing.T) {
	db := newCouponStoreTestDB(t)
	store := gormstore.New(db)
	coupon := createCouponStoreFixture(t, store)

	if err := store.Delete(coupon.ID); err != nil {
		t.Fatalf("delete coupon: %v", err)
	}
	if got, err := store.GetByID(coupon.ID); err != nil || got != nil {
		t.Fatalf("GetByID after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.GetByCode(coupon.Code); err != nil || got != nil {
		t.Fatalf("GetByCode after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.ListByIDs([]uint{coupon.ID}); err != nil || len(got) != 0 {
		t.Fatalf("ListByIDs after delete = (%v, %v), want empty", got, err)
	}
	if got, total, err := store.List(couponcontract.ListFilter{}); err != nil || total != 0 || len(got) != 0 {
		t.Fatalf("List after delete = (%v, %d, %v), want empty", got, total, err)
	}
	if err := store.IncrementUsedCount(coupon.ID, 1); err != nil {
		t.Fatalf("increment deleted coupon: %v", err)
	}

	var raw coupondomain.Coupon
	if err := db.First(&raw, coupon.ID).Error; err != nil {
		t.Fatalf("load raw deleted coupon: %v", err)
	}
	if raw.DeletedAt == nil {
		t.Fatal("deleted coupon must retain a non-nil deleted_at marker")
	}
	if raw.UsedCount != 2 {
		t.Fatalf("deleted coupon used_count changed to %d", raw.UsedCount)
	}
}

func TestCouponUsageStoreSoftDeleteHidesUsageFromEveryReadPath(t *testing.T) {
	db := newCouponStoreTestDB(t)
	store := gormstore.NewUsageStore(db)
	usage := &coupondomain.CouponUsage{
		CouponID:       10,
		UserID:         20,
		OrderID:        30,
		DiscountAmount: money.FromDecimal(decimal.NewFromInt(5)),
	}
	if err := store.Create(usage); err != nil {
		t.Fatalf("create usage: %v", err)
	}
	if err := store.DeleteByOrderID(usage.OrderID); err != nil {
		t.Fatalf("delete usage: %v", err)
	}

	if got, err := store.CountByUser(usage.CouponID, usage.UserID); err != nil || got != 0 {
		t.Fatalf("CountByUser after delete = (%d, %v), want zero", got, err)
	}
	if got, err := store.ListByOrderID(usage.OrderID); err != nil || len(got) != 0 {
		t.Fatalf("ListByOrderID after delete = (%v, %v), want empty", got, err)
	}
	if got, total, err := store.ListByUser(couponcontract.UsageListFilter{UserID: usage.UserID}); err != nil || total != 0 || len(got) != 0 {
		t.Fatalf("ListByUser after delete = (%v, %d, %v), want empty", got, total, err)
	}

	var raw coupondomain.CouponUsage
	if err := db.First(&raw, usage.ID).Error; err != nil {
		t.Fatalf("load raw deleted usage: %v", err)
	}
	if raw.DeletedAt == nil {
		t.Fatal("deleted usage must retain a non-nil deleted_at marker")
	}
}

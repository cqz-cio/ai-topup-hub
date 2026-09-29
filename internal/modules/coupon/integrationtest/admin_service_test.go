package integrationtest

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	couponapp "github.com/dujiao-next/internal/modules/coupon/application"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	coupongormstore "github.com/dujiao-next/internal/modules/coupon/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newCouponAdminServiceForTest(t *testing.T) (*couponapp.AdminService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:coupon_admin_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&coupondomain.Coupon{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return couponapp.NewAdminService(coupongormstore.New(db)), db
}

func validCouponAdminInput(code string) couponapp.CreateCouponInput {
	return couponapp.CreateCouponInput{
		Code:        code,
		Type:        constants.CouponTypePercent,
		Value:       money.FromDecimal(decimal.NewFromInt(10)),
		MinAmount:   money.FromDecimal(decimal.Zero),
		MaxDiscount: money.FromDecimal(decimal.Zero),
		ScopeRefIDs: []uint{1},
	}
}

func TestCouponAdminServiceCreateDefaultsDisabledWholesalePriceFalse(t *testing.T) {
	svc, _ := newCouponAdminServiceForTest(t)

	coupon, err := svc.Create(validCouponAdminInput("DEFAULT_WHOLESALE"))
	if err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}
	if coupon.DisabledWholesalePrice {
		t.Fatalf("disabled_wholesale_price should default to false")
	}
	if coupon.PerItemDiscount {
		t.Fatalf("per_item_discount should default to false")
	}
}

func TestCouponAdminServiceCreateAndUpdateDisabledWholesalePrice(t *testing.T) {
	svc, _ := newCouponAdminServiceForTest(t)

	disabled := true
	input := validCouponAdminInput("DISABLED_WHOLESALE")
	input.DisabledWholesalePrice = &disabled
	coupon, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}
	if !coupon.DisabledWholesalePrice {
		t.Fatalf("disabled_wholesale_price should be true after create")
	}

	updateInput := couponapp.UpdateCouponInput(validCouponAdminInput("DISABLED_WHOLESALE_RENAMED"))
	updated, err := svc.Update(coupon.ID, updateInput)
	if err != nil {
		t.Fatalf("update coupon failed: %v", err)
	}
	if !updated.DisabledWholesalePrice {
		t.Fatalf("disabled_wholesale_price should be preserved when update input omits it")
	}

	disabled = false
	updateInput.DisabledWholesalePrice = &disabled
	updated, err = svc.Update(coupon.ID, updateInput)
	if err != nil {
		t.Fatalf("update coupon failed: %v", err)
	}
	if updated.DisabledWholesalePrice {
		t.Fatalf("disabled_wholesale_price should be false after explicit update")
	}
}

func TestCouponAdminServiceCreateAndUpdatePerItemDiscount(t *testing.T) {
	svc, _ := newCouponAdminServiceForTest(t)

	enabled := true
	input := validCouponAdminInput("PER_ITEM")
	input.Type = constants.CouponTypeFixed
	input.PerItemDiscount = &enabled
	coupon, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}
	if !coupon.PerItemDiscount {
		t.Fatalf("per_item_discount should be true after fixed coupon create")
	}

	updateInput := couponapp.UpdateCouponInput(validCouponAdminInput("PER_ITEM_RENAMED"))
	updateInput.Type = constants.CouponTypeFixed
	updated, err := svc.Update(coupon.ID, updateInput)
	if err != nil {
		t.Fatalf("update coupon failed: %v", err)
	}
	if !updated.PerItemDiscount {
		t.Fatalf("per_item_discount should be preserved when update input omits it")
	}

	enabled = false
	updateInput.PerItemDiscount = &enabled
	updated, err = svc.Update(coupon.ID, updateInput)
	if err != nil {
		t.Fatalf("update coupon failed: %v", err)
	}
	if updated.PerItemDiscount {
		t.Fatalf("per_item_discount should be false after explicit update")
	}
}

func TestCouponAdminServiceIgnoresPerItemDiscountForPercentCoupon(t *testing.T) {
	svc, _ := newCouponAdminServiceForTest(t)

	enabled := true
	input := validCouponAdminInput("PER_ITEM_PERCENT")
	input.PerItemDiscount = &enabled
	coupon, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}
	if coupon.PerItemDiscount {
		t.Fatalf("per_item_discount should be false for percent coupon")
	}

	updateInput := couponapp.UpdateCouponInput(validCouponAdminInput("PER_ITEM_PERCENT_RENAMED"))
	updateInput.PerItemDiscount = &enabled
	updated, err := svc.Update(coupon.ID, updateInput)
	if err != nil {
		t.Fatalf("update coupon failed: %v", err)
	}
	if updated.PerItemDiscount {
		t.Fatalf("per_item_discount should be false after percent coupon update")
	}
}

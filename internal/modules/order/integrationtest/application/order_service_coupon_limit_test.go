package application_test

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	coupongormstore "github.com/dujiao-next/internal/modules/coupon/infrastructure/gormstore"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	usergormstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	. "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"
	promotiongormstore "github.com/dujiao-next/internal/modules/promotion/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type couponCodeBarrierRepository struct {
	couponcontract.Repository
	reached chan<- struct{}
	release <-chan struct{}
}

func (r *couponCodeBarrierRepository) GetByCode(code string) (*coupondomain.Coupon, error) {
	coupon, err := r.Repository.GetByCode(code)
	if err == nil && coupon != nil {
		r.reached <- struct{}{}
		<-r.release
	}
	return coupon, err
}

type couponUsageBarrierRepository struct {
	couponcontract.UsageRepository
	reached chan<- struct{}
	release <-chan struct{}
}

func (r *couponUsageBarrierRepository) CountByUser(couponID, userID uint) (int64, error) {
	count, err := r.UsageRepository.CountByUser(couponID, userID)
	if err == nil {
		r.reached <- struct{}{}
		<-r.release
	}
	return count, err
}

type concurrentCouponQueue struct {
	enqueued atomic.Int32
}

func (q *concurrentCouponQueue) Enabled() bool { return true }

func (q *concurrentCouponQueue) EnqueueTimeoutCancel(_ uint, _ time.Duration) error {
	q.enqueued.Add(1)
	return nil
}

func (q *concurrentCouponQueue) EnqueueStatusEmail(_ uint, _ string) error { return nil }

type couponLimitRaceFixture struct {
	db         *gorm.DB
	product    productdomain.Product
	sku        productdomain.ProductSKU
	coupon     coupondomain.Coupon
	users      []userdomain.User
	couponRepo couponcontract.Repository
	usageRepo  couponcontract.UsageRepository
	queue      *concurrentCouponQueue
}

func newCouponLimitRaceFixture(t *testing.T, name string, usageLimit, perUserLimit int) couponLimitRaceFixture {
	t.Helper()

	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", name, time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(
		&userdomain.User{},
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
		&promotiondomain.Promotion{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{
		Slug:      name + "-category",
		NameJSON:  jsonmap.JSON{"zh-CN": "优惠券并发测试"},
		IsActive:  true,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            name + "-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "优惠券并发测试商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	sku := productdomain.ProductSKU{
		ProductID:        product.ID,
		SKUCode:          productdomain.DefaultSKUCode,
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		ManualStockTotal: constants.ManualStockUnlimited,
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku: %v", err)
	}
	users := []userdomain.User{
		{Email: name + "-first@example.com", PasswordHash: "hash", Status: "active", CreatedAt: now, UpdatedAt: now},
		{Email: name + "-second@example.com", PasswordHash: "hash", Status: "active", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	coupon := coupondomain.Coupon{
		Code:         name + "-coupon",
		Type:         constants.CouponTypeFixed,
		Value:        money.FromDecimal(decimal.NewFromInt(10)),
		MinAmount:    money.FromDecimal(decimal.Zero),
		MaxDiscount:  money.FromDecimal(decimal.Zero),
		UsageLimit:   usageLimit,
		PerUserLimit: perUserLimit,
		ScopeType:    constants.ScopeTypeProduct,
		ScopeRefIDs:  fmt.Sprintf("[%d]", product.ID),
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&coupon).Error; err != nil {
		t.Fatalf("create coupon: %v", err)
	}

	return couponLimitRaceFixture{
		db:         db,
		product:    product,
		sku:        sku,
		coupon:     coupon,
		users:      users,
		couponRepo: coupongormstore.New(db),
		usageRepo:  coupongormstore.NewUsageStore(db),
		queue:      &concurrentCouponQueue{},
	}
}

func (f couponLimitRaceFixture) service(coupons couponcontract.Repository, usages couponcontract.UsageRepository) *OrderService {
	return NewOrderService(OrderServiceOptions{
		OrderStore:       ordergormstore.New(f.db, "test-guest-credential-secret-with-32-bytes"),
		UserStore:        usergormstore.New(f.db),
		ProductStore:     productgormstore.NewProductStore(f.db),
		ProductSKUStore:  productgormstore.NewSKUStore(f.db),
		CouponStore:      coupons,
		CouponUsageStore: usages,
		PromotionRepo:    promotiongormstore.New(f.db),
		Queue:            f.queue,
		ExpireMinutes:    15,
	})
}

func (f couponLimitRaceFixture) input(userID uint) CreateOrderInput {
	return CreateOrderInput{
		UserID:     userID,
		CouponCode: f.coupon.Code,
		Items: []CreateOrderItem{{
			ProductID: f.product.ID,
			SKUID:     f.sku.ID,
			Quantity:  1,
		}},
	}
}

func assertConcurrentCouponLimit(t *testing.T, f couponLimitRaceFixture, svc *OrderService, userIDs []uint, reached <-chan struct{}, release chan struct{}, wantLimitErr error) {
	t.Helper()
	results := make(chan error, len(userIDs))
	for _, userID := range userIDs {
		go func(uid uint) {
			_, err := svc.CreateOrder(f.input(uid))
			results <- err
		}(userID)
	}

	for range userIDs {
		select {
		case <-reached:
		case <-time.After(5 * time.Second):
			t.Fatal("orders did not reach the coupon pre-check together")
		}
	}
	close(release)

	var succeeded, rejected int
	for range userIDs {
		select {
		case err := <-results:
			switch {
			case err == nil:
				succeeded++
			case errors.Is(err, wantLimitErr):
				rejected++
			default:
				t.Fatalf("unexpected CreateOrder error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent CreateOrder calls did not finish")
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("want one success and one limit rejection, got success=%d rejected=%d", succeeded, rejected)
	}

	var coupon coupondomain.Coupon
	if err := f.db.First(&coupon, f.coupon.ID).Error; err != nil {
		t.Fatalf("reload coupon: %v", err)
	}
	if coupon.UsedCount != 1 {
		t.Fatalf("coupon used_count = %d, want 1", coupon.UsedCount)
	}
	var usageCount int64
	if err := f.db.Model(&coupondomain.CouponUsage{}).Where("coupon_id = ?", f.coupon.ID).Count(&usageCount).Error; err != nil {
		t.Fatalf("count coupon usages: %v", err)
	}
	if usageCount != 1 {
		t.Fatalf("coupon usage count = %d, want 1", usageCount)
	}
	if f.queue.enqueued.Load() != 1 {
		t.Fatalf("queued order expirations = %d, want 1", f.queue.enqueued.Load())
	}
}

func TestCreateOrderSerializesCouponUsageLimit(t *testing.T) {
	f := newCouponLimitRaceFixture(t, "coupon_total_limit_race", 1, 0)
	reached := make(chan struct{}, 2)
	release := make(chan struct{})
	coupons := &couponCodeBarrierRepository{Repository: f.couponRepo, reached: reached, release: release}

	assertConcurrentCouponLimit(
		t,
		f,
		f.service(coupons, f.usageRepo),
		[]uint{f.users[0].ID, f.users[1].ID},
		reached,
		release,
		couponcontract.ErrUsageLimit,
	)
}

func TestCreateOrderSerializesCouponPerUserLimit(t *testing.T) {
	f := newCouponLimitRaceFixture(t, "coupon_per_user_limit_race", 0, 1)
	reached := make(chan struct{}, 2)
	release := make(chan struct{})
	usages := &couponUsageBarrierRepository{UsageRepository: f.usageRepo, reached: reached, release: release}

	assertConcurrentCouponLimit(
		t,
		f,
		f.service(f.couponRepo, usages),
		[]uint{f.users[0].ID, f.users[0].ID},
		reached,
		release,
		couponcontract.ErrPerUserLimit,
	)
}

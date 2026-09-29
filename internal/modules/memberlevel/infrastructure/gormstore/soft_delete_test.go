package gormstore

import (
	"fmt"
	"testing"
	"time"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newMemberLevelStoreTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:memberlevel_store_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&memberleveldomain.MemberLevel{}, &memberleveldomain.MemberLevelPrice{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func createStoreTestLevel(t *testing.T, db *gorm.DB) memberleveldomain.MemberLevel {
	t.Helper()

	level := memberleveldomain.MemberLevel{
		NameJSON:          jsonmap.JSON{"zh-CN": "VIP"},
		Slug:              "vip",
		DiscountRate:      money.FromDecimal(decimal.NewFromInt(90)),
		RechargeThreshold: money.FromDecimal(decimal.NewFromInt(100)),
		SpendThreshold:    money.FromDecimal(decimal.NewFromInt(100)),
		IsDefault:         true,
		SortOrder:         10,
		IsActive:          true,
	}
	if err := db.Create(&level).Error; err != nil {
		t.Fatalf("create level: %v", err)
	}
	return level
}

func TestLevelStoreSoftDeleteHidesLevelFromEveryReadPath(t *testing.T) {
	db := newMemberLevelStoreTestDB(t)
	level := createStoreTestLevel(t, db)
	store := NewLevelStore(db)

	if err := store.Delete(level.ID); err != nil {
		t.Fatalf("delete level: %v", err)
	}

	if got, err := store.GetByID(level.ID); err != nil || got != nil {
		t.Fatalf("GetByID after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.GetBySlug(level.Slug); err != nil || got != nil {
		t.Fatalf("GetBySlug after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.GetDefault(); err != nil || got != nil {
		t.Fatalf("GetDefault after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.GetActiveBySortOrder(level.SortOrder, 0); err != nil || got != nil {
		t.Fatalf("GetActiveBySortOrder after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.ListAllActive(); err != nil || len(got) != 0 {
		t.Fatalf("ListAllActive after delete = (%v, %v), want empty", got, err)
	}
	if got, total, err := store.List(memberlevelcontract.ListFilter{}); err != nil || total != 0 || len(got) != 0 {
		t.Fatalf("List after delete = (%v, %d, %v), want empty", got, total, err)
	}

	var raw memberleveldomain.MemberLevel
	if err := db.First(&raw, level.ID).Error; err != nil {
		t.Fatalf("load raw deleted level: %v", err)
	}
	if raw.DeletedAt == nil {
		t.Fatal("deleted level must retain a non-nil deleted_at marker")
	}
}

func TestPriceStoreSoftDeleteHidesPriceFromEveryReadPath(t *testing.T) {
	db := newMemberLevelStoreTestDB(t)
	level := createStoreTestLevel(t, db)
	price := memberleveldomain.MemberLevelPrice{
		MemberLevelID: level.ID,
		ProductID:     100,
		SKUID:         200,
		PriceAmount:   money.FromDecimal(decimal.NewFromInt(8)),
	}
	if err := db.Create(&price).Error; err != nil {
		t.Fatalf("create price: %v", err)
	}
	store := NewPriceStore(db)

	if err := store.DeleteByProduct(price.ProductID); err != nil {
		t.Fatalf("delete prices by product: %v", err)
	}

	if got, err := store.GetByID(price.ID); err != nil || got != nil {
		t.Fatalf("GetByID after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.GetByLevelAndProductAndSKU(level.ID, price.ProductID, price.SKUID); err != nil || got != nil {
		t.Fatalf("GetByLevelAndProductAndSKU after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.ListByProduct(price.ProductID); err != nil || len(got) != 0 {
		t.Fatalf("ListByProduct after delete = (%v, %v), want empty", got, err)
	}
	if got, err := store.ListByLevelAndProducts(level.ID, []uint{price.ProductID}); err != nil || len(got) != 0 {
		t.Fatalf("ListByLevelAndProducts after delete = (%v, %v), want empty", got, err)
	}

	var raw memberleveldomain.MemberLevelPrice
	if err := db.First(&raw, price.ID).Error; err != nil {
		t.Fatalf("load raw deleted price: %v", err)
	}
	if raw.DeletedAt == nil {
		t.Fatal("deleted price must retain a non-nil deleted_at marker")
	}
}

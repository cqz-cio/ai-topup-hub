package gormstore_test

import (
	"fmt"
	"testing"
	"time"

	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	"github.com/dujiao-next/internal/modules/giftcard/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newGiftCardStoreTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:giftcard_store_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&giftcarddomain.GiftCardBatch{}, &giftcarddomain.GiftCard{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestGiftCardStoreSoftDeleteHidesCardFromEveryReadAndWritePath(t *testing.T) {
	db := newGiftCardStoreTestDB(t)
	store := gormstore.New(db)
	batch := &giftcarddomain.GiftCardBatch{
		BatchNo:  "GCB-DELETE",
		Name:     "待删除批次",
		Amount:   money.FromDecimal(decimal.NewFromInt(10)),
		Currency: "CNY",
		Quantity: 1,
	}
	cards := []giftcarddomain.GiftCard{{
		Name:     "待删除礼品卡",
		Code:     "GC-DELETE-001",
		Amount:   money.FromDecimal(decimal.NewFromInt(10)),
		Currency: "CNY",
		Status:   giftcarddomain.GiftCardStatusActive,
	}}
	if err := store.CreateBatch(batch, cards); err != nil {
		t.Fatalf("create batch: %v", err)
	}
	var card giftcarddomain.GiftCard
	if err := db.Where("batch_id = ?", batch.ID).First(&card).Error; err != nil {
		t.Fatalf("load created card: %v", err)
	}

	if err := store.Delete(card.ID); err != nil {
		t.Fatalf("delete card: %v", err)
	}
	if got, err := store.GetByID(card.ID); err != nil || got != nil {
		t.Fatalf("GetByID after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.GetByCodeForUpdate(card.Code); err != nil || got != nil {
		t.Fatalf("GetByCodeForUpdate after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.ListByIDs([]uint{card.ID}); err != nil || len(got) != 0 {
		t.Fatalf("ListByIDs after delete = (%v, %v), want empty", got, err)
	}
	if got, total, err := store.List(giftcardcontract.ListFilter{}); err != nil || total != 0 || len(got) != 0 {
		t.Fatalf("List after delete = (%v, %d, %v), want empty", got, total, err)
	}
	if affected, err := store.BatchUpdateStatus([]uint{card.ID}, giftcarddomain.GiftCardStatusDisabled, time.Now()); err != nil || affected != 0 {
		t.Fatalf("BatchUpdateStatus after delete = (%d, %v), want zero", affected, err)
	}

	var raw giftcarddomain.GiftCard
	if err := db.First(&raw, card.ID).Error; err != nil {
		t.Fatalf("load raw deleted card: %v", err)
	}
	if raw.DeletedAt == nil {
		t.Fatal("deleted card must retain a non-nil deleted_at marker")
	}
	if raw.Status != giftcarddomain.GiftCardStatusActive {
		t.Fatalf("deleted card status changed to %s", raw.Status)
	}
}

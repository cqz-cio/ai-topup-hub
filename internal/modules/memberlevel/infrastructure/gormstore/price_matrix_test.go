package gormstore

import (
	"errors"
	"testing"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func priceMatrixDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newMemberLevelStoreTestDB(t)
	for _, sql := range []string{
		"CREATE TABLE products (id integer primary key, deleted_at datetime)",
		"CREATE TABLE product_skus (id integer primary key, product_id integer, deleted_at datetime)",
		"INSERT INTO products(id) VALUES (100), (200)",
		"INSERT INTO product_skus(id, product_id) VALUES (101,100), (201,200)",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func matrixPrice(product, sku uint, amount string) memberleveldomain.MemberLevelPrice {
	return memberleveldomain.MemberLevelPrice{MemberLevelID: 1, ProductID: product, SKUID: sku, PriceAmount: money.FromDecimal(decimal.RequireFromString(amount))}
}

func TestReplacePricesClearsRestoresAndIsolatesProducts(t *testing.T) {
	db := priceMatrixDB(t)
	store := NewPriceStore(db)
	if err := store.BatchUpsert([]memberleveldomain.MemberLevelPrice{matrixPrice(100, 101, "8"), matrixPrice(200, 201, "9")}); err != nil {
		t.Fatal(err)
	}
	original, _ := store.GetByLevelAndProductAndSKU(1, 100, 101)
	if err := store.ReplaceByProduct(100, nil); err != nil {
		t.Fatal(err)
	}
	if rows, _ := store.ListByProduct(100); len(rows) != 0 {
		t.Fatal("cleared overrides still visible")
	}
	if err := store.ReplaceByProduct(100, []memberleveldomain.MemberLevelPrice{matrixPrice(100, 101, "7")}); err != nil {
		t.Fatal(err)
	}
	price, _ := store.GetByLevelAndProductAndSKU(1, 100, 101)
	if price == nil || price.ID != original.ID || price.PriceAmount.String() != "7.00" {
		t.Fatalf("price not restored: %+v", price)
	}
	other, _ := store.GetByLevelAndProductAndSKU(1, 200, 201)
	if other == nil || other.PriceAmount.String() != "9.00" {
		t.Fatal("another product changed")
	}
}

func TestReplacePricesRejectsForeignVariantAndRollsBackFailedSave(t *testing.T) {
	db := priceMatrixDB(t)
	store := NewPriceStore(db)
	if err := store.BatchUpsert([]memberleveldomain.MemberLevelPrice{matrixPrice(100, 101, "8")}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceByProduct(100, []memberleveldomain.MemberLevelPrice{matrixPrice(100, 201, "7")}); !errors.Is(err, memberlevelcontract.ErrPriceInvalid) {
		t.Fatalf("foreign SKU accepted: %v", err)
	}
	if err := db.Exec("CREATE TRIGGER reject_price BEFORE INSERT ON member_level_prices WHEN NEW.price_amount = 99 BEGIN SELECT RAISE(ABORT, 'fixture failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceByProduct(100, []memberleveldomain.MemberLevelPrice{matrixPrice(100, 101, "7"), matrixPrice(100, 0, "99")}); err == nil {
		t.Fatal("expected save failure")
	}
	price, _ := store.GetByLevelAndProductAndSKU(1, 100, 101)
	if price == nil || price.PriceAmount.String() != "8.00" {
		t.Fatalf("failed replacement changed old price: %+v", price)
	}
}

func TestReplacePricesByLevelRestoresRowsAndRollsBackFailure(t *testing.T) {
	db := priceMatrixDB(t)
	level := createStoreTestLevel(t, db)
	other := level
	other.ID = 0
	other.Slug = "other-grade"
	other.SortOrder = 20
	other.IsDefault = false
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	store := NewPriceStore(db)
	otherPrice := matrixPrice(100, 101, "6")
	otherPrice.MemberLevelID = other.ID
	if err := store.BatchUpsert([]memberleveldomain.MemberLevelPrice{matrixPrice(100, 101, "8"), matrixPrice(200, 201, "9"), otherPrice}); err != nil {
		t.Fatal(err)
	}
	original, _ := store.GetByLevelAndProductAndSKU(level.ID, 100, 101)
	if err := store.ReplaceByLevel(level.ID, nil); err != nil {
		t.Fatal(err)
	}
	if rows, err := store.ListByLevel(level.ID); err != nil || len(rows) != 0 {
		t.Fatal("cleared rows still visible", err)
	}
	if rows, err := store.ListByLevel(other.ID); err != nil || len(rows) != 1 {
		t.Fatal("other level was changed", err)
	}
	if err := store.ReplaceByLevel(level.ID, []memberleveldomain.MemberLevelPrice{matrixPrice(100, 101, "7")}); err != nil {
		t.Fatal(err)
	}
	restored, _ := store.GetByLevelAndProductAndSKU(level.ID, 100, 101)
	if restored == nil || restored.ID != original.ID || restored.PriceAmount.String() != "7.00" {
		t.Fatal("row was not restored", restored)
	}
	for _, invalid := range []memberleveldomain.MemberLevelPrice{otherPrice, matrixPrice(100, 201, "9"), matrixPrice(999, 0, "9")} {
		if err := store.ReplaceByLevel(level.ID, []memberleveldomain.MemberLevelPrice{invalid}); !errors.Is(err, memberlevelcontract.ErrPriceInvalid) {
			t.Fatal("invalid scope accepted", err)
		}
	}
	if err := db.Exec("CREATE TRIGGER reject_level_price BEFORE INSERT ON member_level_prices WHEN NEW.price_amount = 99 BEGIN SELECT RAISE(ABORT, 'fixture failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceByLevel(level.ID, []memberleveldomain.MemberLevelPrice{matrixPrice(100, 101, "5"), matrixPrice(200, 0, "99")}); err == nil {
		t.Fatal("expected transaction failure")
	}
	remaining, _ := store.GetByLevelAndProductAndSKU(level.ID, 100, 101)
	if remaining == nil || remaining.PriceAmount.String() != "7.00" {
		t.Fatal("transaction did not roll back")
	}
}
